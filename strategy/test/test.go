package test

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"syscall"

	"github.com/dafsic/hunter/exchange/binance"
	"github.com/dafsic/hunter/exchange/model"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/valyala/fastjson"
)

type Stratety interface {
	Start()
}

type Test struct {
	binanceSpot *binance.BinanceSpotExchange
	l           log.Logger
}

func NewTest(l log.Logger, binanceSpot *binance.BinanceSpotExchange) *Test {
	return &Test{
		l:           l,
		binanceSpot: binanceSpot,
	}
}

func (t *Test) Start() {
	symbolInfo := t.binanceSpot.GetStructOfAllSymbolInfo()

	// 获取所有交易对
	symbolNames, ok := symbolInfo.GetAllSymbolName(model.BinanceSpot)
	if !ok {
		t.l.Log(slog.LevelError, "获取交易所信息失败")
		Exit()
	}

	symbolNames = symbolNames[:300]
	fmt.Println("交易对数量", len(symbolNames))
	for i := range symbolNames {
		if symbolNames[i] == model.SymbolName("BTC-USDT") {
			symbolNames = append(symbolNames[:i], symbolNames[i+1:]...)
			break
		}
	}
	symbolNames = append(symbolNames, model.SymbolName("BTC-USDT"))

	var allUpdateID = make(map[string]*updateID)
	infos, ok := symbolInfo.GetAllSymbol(model.BinanceSpot)
	if !ok {
		t.l.Log(slog.LevelError, "获取交易所信息失败")
		Exit()
	}
	for _, info := range infos {
		allUpdateID[info.NameInExchange] = newUpdateID()
	}
	callback := func(msg []byte) {
		nameInExchang := fastjson.GetString(msg, "data", "s")
		if nameInExchang != "BTCUSDT" {
			return
		}
		updateID := fastjson.GetInt(msg, "data", "u")

		ok := allUpdateID[nameInExchang].Keep(updateID)
		if ok {
			t.l.Log(slog.LevelDebug, nameInExchang+"   "+strconv.Itoa(updateID))
		}
	}

	t.l.Log(slog.LevelInfo, "单网卡单IP多连接")
	// 订阅最优挂单 (普通模式)
	for i := 0; i < 20; i++ {
		err := t.binanceSpot.SubBookTicker(symbolNames, callback, true)
		if err != nil {
			t.l.Log(slog.LevelError, "获取交易所信息失败")
			Exit()
			return
		}
	}
}

func Exit() {
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
