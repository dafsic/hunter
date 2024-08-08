package test

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/pkg/log"
)

type Stratety interface {
	Start()
	Active()
}

type Test struct {
	binanceSpot       exchange.Exchange
	l                 log.Logger
	apiKey            string
	secretKey         string
	ed25519ApiKey     string
	ed25519PrivateKey ed25519.PrivateKey
}

func NewTest(l log.Logger, binanceSpot exchange.Exchange, cfg config.BinanceCfg) (*Test, error) {
	t := &Test{
		l:             l,
		binanceSpot:   binanceSpot,
		apiKey:        cfg.ApiKey,
		secretKey:     cfg.SecretKey,
		ed25519ApiKey: cfg.Ed25519ApiKey,
	}

	// 解析ed25519私钥
	block, _ := pem.Decode([]byte(cfg.Ed25519PrivateKey))
	if block == nil {
		return nil, errors.New("ed25519PrivateKey error")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	t.ed25519PrivateKey = priv.(ed25519.PrivateKey)

	return t, nil
}

func (t *Test) Start() {
	t.l.Info("test strategy start")

	var symbolNames = []exchange.SymbolName{
		exchange.SymbolName("BTC-USDT"),
		exchange.SymbolName("ETH-USDT"),
		exchange.SymbolName("BNB-USDT"),
		exchange.SymbolName("ADA-USDT"),
		exchange.SymbolName("XRP-USDT"),
		exchange.SymbolName("DOGE-USDT"),
		exchange.SymbolName("DOT-USDT"),
	}

	t.l.Debug("交易对数量", len(symbolNames))
	for i := range symbolNames {
		if symbolNames[i] == exchange.SymbolName("BTC-USDT") {
			symbolNames = append(symbolNames[:i], symbolNames[i+1:]...)
			break
		}
	}
	symbolNames = append(symbolNames, exchange.SymbolName("BTC-USDT"))

	var allUpdateID = make(map[exchange.SymbolName]*updateID)
	for _, s := range symbolNames {
		info := t.binanceSpot.GetSymbolInfo(s)
		allUpdateID[info.SymbolName] = newUpdateID()
	}

	callback := func(bookTicket *exchange.BookTicker) {
		name, ask, askQty, bid, bidQty, uID, uTime := bookTicket.Get()
		if name != "BTCUSDT" {
			return
		}

		ok := allUpdateID[name].Keep(int(uID))
		if ok {
			t.l.Debug(fmt.Sprintf("name: %s, ask: %s %s bid: %s %s, updateID: %d updateTime:%d", name, ask, askQty, bid, bidQty, uID, uTime))
		}
	}

	// 订阅最优挂单 (普通模式)
	mc, err := t.binanceSpot.CreateMarketController(symbolNames, "")
	if err != nil {
		t.l.Error("获取交易所信息失败")
		return
	}

	MarketChan := mc.BookTickerChannel()
	for {
		select {
		case mgs := <-MarketChan:
			callback(mgs)
		}
	}
}

// func Exit() {
// 	syscall.Kill(os.Getpid(), syscall.SIGINT)
// }

func (t *Test) Active() {}
