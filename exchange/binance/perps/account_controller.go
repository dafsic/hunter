package perps

import (
	"fmt"
	"sync"
	"time"

	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/pkg/ws"
	"github.com/dafsic/hunter/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
)

type PerpsAccountController struct {
	e            exchange.Exchange
	client       ws.Client
	rLock        sync.RWMutex
	stopC        chan struct{}
	balanceC     chan *exchange.Balance
	orderUpdateC chan *exchange.OrderUpdate
}

func NewPerpsAccountController(e exchange.Exchange, url, apiKey, localIP string, heartBeat int) (*PerpsAccountController, error) {
	ac := &PerpsAccountController{
		e:            e,
		stopC:        make(chan struct{}),
		balanceC:     make(chan *exchange.Balance),
		orderUpdateC: make(chan *exchange.OrderUpdate),
	}
	listenKey, err := ac.CreateLisenKey(apiKey)
	if err != nil {
		return nil, err
	}
	url = url + "/ws/" + listenKey

	client, err := e.WebSocketManager().NewClient(url, localIP, ac.callback, heartBeat, true)
	if err != nil {
		return nil, err
	}
	ac.client = client

	// 处理断线重连
	go func() {
		var (
			disconnectC <-chan struct{}
			err         error
		)
		disconnectC = ac.client.IsDisconnect()
		for {
			select {
			case <-ac.stopC:
				return
			case <-disconnectC:
				ac.rLock.Lock()
				for {
					client, err = e.WebSocketManager().NewClient(url, localIP, ac.callback, heartBeat, true)
					if err != nil {
						e.Logger().Warn("websocket reconnected fail", "url", url, "error", err)
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}
				disconnectC = client.IsDisconnect()
				ac.rLock.Unlock()
			}
		}
	}()

	return ac, nil
}

func (ac *PerpsAccountController) Close() {
	ac.rLock.Lock()
	defer ac.rLock.Unlock()

	close(ac.stopC)
	ac.client.Close()
}

func (ac *PerpsAccountController) BalanceChannel() <-chan *exchange.Balance {
	return ac.balanceC
}

func (ac *PerpsAccountController) OrderUpdateChannel() <-chan *exchange.OrderUpdate {
	return ac.orderUpdateC
}

func (ac *PerpsAccountController) CreateLisenKey(apiKey string) (string, error) {
	// 获取合约LisenKey
	urlPath := ac.e.GetBaseUrl() + "/fapi/v1/listenKey"
	_, res, err := ac.e.SendWithApikey(urlPath, "POST", nil, apiKey)
	if err != nil {
		return "", fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	var lisenKeyData struct {
		ListenKey string `json:"listenKey"`
	}

	err = jsoniter.Unmarshal(res, &lisenKeyData)
	if err != nil {
		return "", fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	go func() {
		// 更新 LisenKey 有效期
		tick := time.NewTicker(10 * time.Minute)
		defer tick.Stop()

		for {
			select {
			case <-ac.stopC:
				return
			case <-tick.C:
				_, _, err = ac.e.SendWithApikey(urlPath, "PUT", map[string]string{"listenKey": lisenKeyData.ListenKey}, apiKey)
				if err != nil {
					ac.e.Logger().Warn("refresh lisenKey error", "error", err.Error())
				}
			}
		}
	}()

	return lisenKeyData.ListenKey, nil
}

func (ac *PerpsAccountController) callback(msg []byte) {
	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		ac.e.Logger().Warn("parse ws message error", "error", err.Error())
		return
	}

	even := string(data.GetStringBytes("e"))
	switch even {
	case "outboundAccountPosition":
		// {
		// 	"e": "outboundAccountPosition", // 事件类型
		// 	"E": 1564034571105,             // 事件时间
		// 	"u": 1564034571073,             // 账户末次更新时间戳
		// 	"B": [                          // 余额
		// 		{
		// 			"a": "ETH",                 // 资产名称
		// 			"f": "10000.000000",        // 可用余额
		// 			"l": "0.000000"             // 冻结余额
		// 		}
		// 	]
		// }

		for _, v := range data.GetArray("B") {
			asset := string(v.GetStringBytes("a"))
			free := utils.StringToFloat64(string(v.GetStringBytes("f")))
			locked := utils.StringToFloat64(string(v.GetStringBytes("l")))
			ac.balanceC <- &exchange.Balance{Asset: asset, Free: free, Locked: locked}
		}

	case "executionReport":
		// {
		// 	"e": "executionReport",        // 事件类型
		// 	"E": 1499405658658,            // 事件时间
		// 	"s": "ETHBTC",                 // 交易对
		// 	"c": "mUvoqJxFIILMdfAW5iGSOW", // clientOrderId
		// 	"S": "BUY",                    // 订单方向
		// 	"o": "LIMIT",                  // 订单类型
		// 	"f": "GTC",                    // 有效方式
		// 	"q": "1.00000000",             // 订单原始数量
		// 	"p": "0.10264410",             // 订单原始价格
		// 	"P": "0.00000000",             // 止盈止损单触发价格
		// 	"F": "0.00000000",             // 冰山订单数量
		// 	"g": -1,                       // OCO订单 OrderListId
		// 	"C": "",                       // 原始订单自定义ID(原始订单，指撤单操作的对象。撤单本身被视为另一个订单)
		// 	"x": "NEW",                    // 本次事件的具体执行类型
		// 	"X": "NEW",                    // 订单的当前状态
		// 	"r": "NONE",                   // 订单被拒绝的原因
		// 	"i": 4293153,                  // orderId
		// 	"l": "0.00000000",             // 订单末次成交量
		// 	"z": "0.00000000",             // 订单累计已成交量
		// 	"L": "0.00000000",             // 订单末次成交价格
		// 	"n": "0",                      // 手续费数量
		// 	"N": null,                     // 手续费资产类别
		// 	"T": 1499405658657,            // 成交时间
		// 	"I": 8641984,                  // 请忽略
		// 	"w": true,                     // 订单是否在订单簿上？
		// 	"m": false,                    // 该成交是作为挂单成交吗？
		// 	"M": false,                    // 请忽略
		// 	"O": 1499405658657,            // 订单创建时间
		// 	"Z": "0.00000000",             // 订单累计已成交金额
		// 	"Y": "0.00000000",             // 订单末次成交金额
		// 	"Q": "0.00000000",             // Quote Order Quantity
		// 	"D": 1668680518494,            // 追踪时间; 这仅在追踪止损订单已被激活时可见
		// 	"W": 1499405658657,            // Working Time; 订单被添加到 order book 的时间
		// 	"V": "NONE"                    // SelfTradePreventionMode
		// }

		nameInExchange := data.GetStringBytes("s")
		orderUpdate := &exchange.OrderUpdate{
			Symbol:     ac.e.GetSymbloInfoByExchangeName(string(nameInExchange)).SymbolName,
			ClientID:   string(data.GetStringBytes("c")),
			TradeQty:   utils.StringToFloat64(string(data.GetStringBytes("l"))),
			TradePrice: utils.StringToFloat64(string(data.GetStringBytes("L"))),
		}

		orderUpdate.UpdateType.Set(string(data.GetStringBytes("x")))

		ac.orderUpdateC <- orderUpdate
	}

	ac.e.Logger().Info("websocket account event", "res", string(msg))
}

var _ exchange.AccountController = (*PerpsAccountController)(nil)
