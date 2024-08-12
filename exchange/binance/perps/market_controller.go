package perps

import (
	"sync"
	"time"

	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/pkg/ws"
	"github.com/dafsic/hunter/utils"
	"github.com/valyala/fastjson"
)

type PerpsMarketController struct {
	e           exchange.Exchange
	client      ws.Client
	rLock       sync.RWMutex
	stopC       chan struct{}
	bookTickerC chan *exchange.BookTicker
}

func NewPerpsMarketController(e exchange.Exchange, url, localIP string, heartBeat int) (*PerpsMarketController, error) {
	mc := &PerpsMarketController{
		e:           e,
		stopC:       make(chan struct{}),
		bookTickerC: make(chan *exchange.BookTicker),
	}

	client, err := e.WebSocketManager().NewClient(url, localIP, mc.callback, heartBeat, true)
	if err != nil {
		return nil, err
	}
	mc.client = client

	// 处理断线重连
	go func() {
		var (
			disconnectC <-chan struct{}
			err         error
		)
		disconnectC = mc.client.IsDisconnect()
		for {
			select {
			case <-mc.stopC:
				return
			case <-disconnectC:
				mc.rLock.Lock()
				for {
					client, err = e.WebSocketManager().NewClient(url, localIP, mc.callback, heartBeat, true)
					if err != nil {
						e.Logger().Warn("websocket reconnected fail", "url", url, "error", err)
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}
				disconnectC = client.IsDisconnect()
				mc.rLock.Unlock()
			}
		}
	}()

	return mc, nil
}

func (mc *PerpsMarketController) Close() {
	mc.rLock.Lock()
	defer mc.rLock.Unlock()

	close(mc.stopC)
	close(mc.bookTickerC)
	mc.client.Close()
}

func (mc *PerpsMarketController) BookTickerChannel() <-chan *exchange.BookTicker {
	return mc.bookTickerC
}

func (mc *PerpsMarketController) callback(msg []byte) {
	// {
	// "stream": "btcusdt@bookTicker",
	// "data": {
	// "u": 49559991096,
	// "s": "BTCUSDT",
	// "b": "64380.00000000",
	// "B": "4.39764000",
	// "a": "64380.01000000",
	// "A": "0.01991000"
	// }
	// }

	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		mc.e.Logger().Warn("market controller callback error", "error", err.Error())
		return
	}

	askPrice := data.GetStringBytes("data", "a")
	askQty := data.GetStringBytes("data", "A")
	bidPrice := data.GetStringBytes("data", "b")
	bidQty := data.GetStringBytes("data", "B")
	updateID := data.GetInt64("data", "u")
	nameInExchange := data.GetStringBytes("data", "s")

	bookTicker := exchange.NewBookTicker(
		mc.e.GetSymbloInfoByExchangeName(string(nameInExchange)).SymbolName,
		utils.StringToFloat64(string(askPrice)),
		utils.StringToFloat64(string(askQty)),
		utils.StringToFloat64(string(bidPrice)),
		utils.StringToFloat64(string(bidQty)),
		updateID,
		0,
	)

	select {
	case mc.bookTickerC <- bookTicker:
	default:
		mc.e.Logger().Warn("book ticker channel is full")
	}
}

var _ exchange.MarketController = (*PerpsMarketController)(nil)
