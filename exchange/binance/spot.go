package binance

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange"
	jcdhttp "github.com/dafsic/hunter/pkg/http"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"

	"github.com/dafsic/hunter/exchange/model"
	"github.com/dafsic/hunter/utils"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
)

type BinanceSpotExchange struct {
	apiKey                  string
	secretKey               string
	ed25519ApiKey           string
	ed25519PrivateKey       ed25519.PrivateKey
	name                    model.ExchangeName         // 交易所名称
	allBalance              *model.AllBalance          // 账户信息
	allPosition             *model.AllPosition         // 持仓信息
	allSymbolInfo           *model.AllSymbolsInfo      // 交易规范
	balanceCallback         func(data *fastjson.Value) // 账户更新回调函数
	positionCallback        func(data *fastjson.Value) // 持仓更新回调函数
	orderCallback           func(data *fastjson.Value) // 订单更新回调函数
	wsOrderClient           *BinanceWsClient           // 下单专用通道
	wsApiClient             *BinanceWsClient           // wsApi发送通道
	wsAccountClient         *BinanceWsClient           // 账户通道
	wsSubBookTickerClients  []*BinanceWsClient         // 订阅最优挂单通道数组
	operateWsReadChanMap    *sync.Map                  // wsApi广播Map  id:chan *fastjson.Value
	orderDetailMap          *model.OrderDetailMap      // 订单映射 (ClientOrderID:*model.OrderDetail)
	rateLimitsMinuteCount   int64                      // IP限速报警阈值
	rateLimitsDayCount      int64                      // 下单1天限速报警阈值
	rateLimitsSecond10Count int64                      // 下单10秒限速报警阈值
	rateLimitC              chan error                 // 限速警报通道
	operateWsUrl            string                     // wsApi wsUrl
	subWsbaseUrl            string                     // 订阅型wsUrl
	wsHeartBeat             int                        // 心跳间隔
	baseHTTPUrl             string                     // baseUrl
	baseHTTPUrlS            []string                   // baseUrl合集
	baseHTTPUrlRlock        *sync.RWMutex              // baseUrl读写锁
	wsManager               ws.Manager                 // ws管理器
	httpManger              jcdhttp.Manager            // http client 管理器
	logger                  log.Logger                 // 日志
	wg                      sync.WaitGroup             // 等待组
	ctx                     context.Context            // 上下文
	cancelFunc              context.CancelFunc         // 用于交易所退出时，通知所有goroutine退出
}

/* ========================= 构建和初始化 ========================= */

func NewBinanceSpot(l log.Logger, wsMgr ws.Manager, httpMgr jcdhttp.Manager, cfg config.BinanceCfg) (*BinanceSpotExchange, error) {
	exchange := &BinanceSpotExchange{
		apiKey:        cfg.ApiKey,
		secretKey:     cfg.SecretKey,
		ed25519ApiKey: cfg.Ed25519ApiKey,
		name:          model.BinanceSpot,

		allBalance:    model.NewAllBalance(),
		allPosition:   model.NewAllPosition(),
		allSymbolInfo: model.NewAllSymbolsInfo(),

		balanceCallback:      func(data *fastjson.Value) {},
		positionCallback:     func(data *fastjson.Value) {},
		orderCallback:        func(data *fastjson.Value) {},
		operateWsReadChanMap: new(sync.Map),
		orderDetailMap:       model.NewOrderDetailMap(),

		rateLimitsMinuteCount:   5900,
		rateLimitsDayCount:      19900,
		rateLimitsSecond10Count: 95,
		rateLimitC:              make(chan error),

		operateWsUrl:     "wss://ws-api.binance.com:443/ws-api/v3",
		subWsbaseUrl:     "wss://stream.binance.com:443",
		wsHeartBeat:      60,
		baseHTTPUrl:      "https://api.binance.com",
		baseHTTPUrlRlock: new(sync.RWMutex),
		baseHTTPUrlS: []string{
			"https://api1.binance.com",
			"https://api2.binance.com",
			"https://api3.binance.com",
			"https://api4.binance.com",
		},
		logger:     l,
		wsManager:  wsMgr,
		httpManger: httpMgr,
	}
	exchange.ctx, exchange.cancelFunc = context.WithCancel(context.Background())

	// 解析ed25519私钥
	if cfg.Ed25519PrivateKey != "" {
		block, _ := pem.Decode([]byte(cfg.Ed25519PrivateKey))
		if block == nil {
			return nil, errors.New("ed25519PrivateKey error")
		}
		priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w%s", err, utils.LineInfo())
		}

		exchange.ed25519PrivateKey = priv.(ed25519.PrivateKey)
	}

	return exchange, nil
}

func (e *BinanceSpotExchange) Init() (err error) {
	e.logger.Log(slog.LevelInfo, "binanceSpot exchange init...")
	defer e.logger.Log(slog.LevelInfo, "binanceSpot exchange init done")
	// 初始化交易规范
	e.allSymbolInfo, err = e.getSymbolInfo()
	if err != nil {
		return fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	if e.apiKey == "" {
		return
	}

	// 建立下单专用连接
	e.wsOrderClient, err = e.NewWsClient(e.operateWsUrl, "", e.operateWsCallback, false)
	if err != nil {
		e.logger.Log(slog.LevelError, "create websocket client error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}
	err = e.wsLogin(e.wsOrderClient)
	if err != nil {
		e.logger.Log(slog.LevelError, "websocket login error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}

	// 建立操作ws连接
	e.wsApiClient, err = e.NewWsClient(e.operateWsUrl, "", e.operateWsCallback, false)
	if err != nil {
		e.logger.Log(slog.LevelError, "create websocket client error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}
	err = e.wsLogin(e.wsOrderClient)
	if err != nil {
		e.logger.Log(slog.LevelError, "websocket login error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}

	// 初始化账户信息
	e.allBalance, err = e.GetHTTPBalance()
	if err != nil {
		e.logger.Log(slog.LevelError, "GetHTTPBalance fail", "error", err)
		return err
	}

	e.wsAccountClient, err = e.NewWsClient(e.subWsbaseUrl+"/ws/"+e.getLisenKey(), "", e.accountWsCallback, true)
	if err != nil {
		e.logger.Log(slog.LevelError, "create websocket client error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}

	// 更新最快api
	go e.updateFasterApi()
	// 更新交易规范
	go e.updateAllSymbolInfo()

	return
}

func (e *BinanceSpotExchange) Exit() {
	e.cancelFunc()
	if e.wsOrderClient != nil {
		e.wsOrderClient.Close()
	}
	if e.wsApiClient != nil {
		e.wsApiClient.Close()
	}
	if e.wsAccountClient != nil {
		e.wsAccountClient.Close()
	}
	for _, v := range e.wsSubBookTickerClients {
		v.Close()
	}
	e.wg.Wait()
	e.logger.Log(slog.LevelInfo, "exchange exit", "exchage", e.name)
}

func (e *BinanceSpotExchange) updateFasterApi() {
	// 更新最快api
	e.wg.Add(1)
	defer e.wg.Done()

	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-tick.C:
			minTime := 3 * time.Second
			for _, vlaue := range e.baseHTTPUrlS {
				urlPath := vlaue + "/api/v3/ping"
				t1 := time.Now()
				e.sendNone(urlPath, "GET", nil, false)
				if time.Since(t1) < minTime {
					minTime = time.Since(t1)
					e.baseHTTPUrlRlock.Lock()
					e.baseHTTPUrl = vlaue
					e.baseHTTPUrlRlock.Unlock()
				}
			}
		}
	}
}

func (e *BinanceSpotExchange) updateAllSymbolInfo() {
	e.wg.Add(1)
	defer e.wg.Done()

	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-tick.C:
			info, err := e.getSymbolInfo()
			if err != nil {
				continue
			}
			e.allSymbolInfo.Update(info)
		}
	}
}

/* ========================= 辅助函数 ========================= */
func (e *BinanceSpotExchange) getBaseUrl() string {
	// 获取现货baseUrl
	e.baseHTTPUrlRlock.RLock()
	defer e.baseHTTPUrlRlock.RUnlock()
	return e.baseHTTPUrl
}

func (e *BinanceSpotExchange) addHttpSign(urlPath string) string {
	// HTTP鉴权请求添加 timestamp、recvWindow、signature 参数
	if urlPath != "" {
		urlPath += "&"
	}
	urlPath += "timestamp=" + strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	urlPath += "&recvWindow=3500"
	urlPath += "&signature=" + utils.GetHamcSha256HexEncodeSignStr(urlPath, e.secretKey)
	return urlPath
}

func (e *BinanceSpotExchange) wsLogin(client *BinanceWsClient) error {
	// WS登录
	params := map[string]string{
		"apiKey":    e.ed25519ApiKey,
		"timestamp": strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
	}
	params["signature"] = utils.GetED25519Base64SignStr([]byte((utils.GetAndEqJionString(params))), e.ed25519PrivateKey)
	data := map[string]any{
		"id":     "login",
		"method": "session.logon",
		"params": params,
	}

	payload, err := jsoniter.Marshal(data)
	if err != nil {
		return err
	}

	err = client.SendMessage(payload)
	if err != nil {
		return err
	}

	return nil
}

func (e *BinanceSpotExchange) operateWsCallback(msg []byte) {
	// websocket API 的回调函数
	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		e.logger.Log(slog.LevelWarn, "parse ws message error", "error", err.Error())
		return
	}

	id := string(data.GetStringBytes("id"))

	if c, ok := e.operateWsReadChanMap.Load(id); ok {
		c.(chan *fastjson.Value) <- data
		e.operateWsReadChanMap.Delete(id)
	}

	for _, rateLimits := range data.GetArray("rateLimits") {

		count := rateLimits.GetInt64("count")
		switch string(rateLimits.GetStringBytes("interval")) {
		case "SECOND":
			if count > e.rateLimitsSecond10Count {
				e.rateLimitC <- model.ErrOrderRateLimits
				e.logger.Log(slog.LevelWarn, "websocket api rate limits", "error", "SECOND 10秒限速超限", "count", count, "exchange", e.name)
				//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=SECOND 10s限速超限	count="+strconv.FormatInt(count, 10)+"	exchange="+string(e.name))
			}
		case "DAY":
			if count > e.rateLimitsDayCount {
				e.rateLimitC <- model.ErrOrderRateLimits
				e.logger.Log(slog.LevelWarn, "websocket api rate limits", "error", "DAY 1天限速超限", "count", count, "exchange", e.name)
				//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=DAY 1天限速超限	count="+strconv.FormatInt(count, 10)+"	exchange="+string(e.name))
			}
		case "MINUTE":
			if count > e.rateLimitsMinuteCount {
				e.rateLimitC <- model.ErrIpRateLimits
				e.logger.Log(slog.LevelWarn, "websocket api rate limits", "error", "MITNUTE 1分钟限速超限", "count", count, "exchange", e.name)
				//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	count="+strconv.FormatInt(count, 10)+"	exchange="+string(e.name))
			}
		}
	}

	e.logger.Log(slog.LevelInfo, "websocket api event", "res", string(msg))
}

func (e *BinanceSpotExchange) accountWsCallback(msg []byte) {
	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		e.logger.Log(slog.LevelWarn, "WS ParseBytes error", "error", err)
		return
	}

	even := string(data.GetStringBytes("e"))
	switch even {
	case "outboundAccountPosition":

		e.balanceCallback(data)

		for _, v := range data.GetArray("B") {
			asset := string(v.GetStringBytes("a"))
			free := utils.StringToFloat64(string(v.GetStringBytes("f")))
			locked := utils.StringToFloat64(string(v.GetStringBytes("l")))
			e.allBalance.Update(asset, &model.Balance{Free: free, Locked: locked})
		}

	case "executionReport":
		e.orderCallback(data)
	}

	e.logger.Log(slog.LevelInfo, "websocket account event", "res", string(msg))
}

func (e *BinanceSpotExchange) getLisenKey() (lisenKey string) {
	// 获取现货LisenKey
	urlPath := e.getBaseUrl() + "/api/v3/userDataStream"
	_, res, err := e.sendApik(urlPath, "POST", nil, false)
	if err != nil {
		return ""
	}

	var lisenKeyData struct {
		ListenKey string `json:"listenKey"`
	}

	err = jsoniter.Unmarshal(res, &lisenKeyData)
	if err != nil {
		return ""
	}
	go e.lisenKeyHearbeat(lisenKeyData.ListenKey)
	return lisenKeyData.ListenKey
}

func (e *BinanceSpotExchange) lisenKeyHearbeat(lisenKey string) {
	e.wg.Add(1)
	defer e.wg.Done()

	// 更新 LisenKey 有效期
	tick := time.NewTicker(10 * time.Minute)
	defer tick.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-tick.C:
			urlPath := e.getBaseUrl() + "/api/v3/userDataStream"
			e.sendApik(urlPath, "PUT", map[string]string{"listenKey": lisenKey}, false)
		}
	}
}

func (e *BinanceSpotExchange) getSymbolInfo() (allSymbolInfo *model.AllSymbolsInfo, err error) {
	var (
		parser  fastjson.Parser
		urlPath = e.getBaseUrl() + "/api/v3/exchangeInfo"
		res     []byte
	)

	_, res, err = e.sendNone(urlPath, "GET", nil, false)
	if err != nil {
		e.logger.Log(slog.LevelWarn, "getSymbolInfo error", "error", err)
		return
	}

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Log(slog.LevelWarn, "getSymbolInfo error", "error", err)
		return
	}

	allSymbolInfo = model.NewAllSymbolsInfo()

	for _, symbol := range data.GetArray("symbols") {
		if string(symbol.GetStringBytes("status")) != "TRADING" {
			continue
		}

		var (
			nameInExchange string
			baseAsset      string
			quoteAsset     string
			symbolName     model.SymbolName

			pricePrecision    int32
			quantityPrecision int32
			minValue          float64
		)

		for _, filter := range symbol.GetArray("filters") {
			if string(filter.GetStringBytes("filterType")) == "LOT_SIZE" {
				quantityPrecision = utils.GetDecimalPlaces(string(filter.GetStringBytes("stepSize")))
			} else if string(filter.GetStringBytes("filterType")) == "PRICE_FILTER" {
				pricePrecision = utils.GetDecimalPlaces(string(filter.GetStringBytes("tickSize")))
			} else if string(filter.GetStringBytes("filterType")) == "NOTIONAL" {
				minValue, _ = strconv.ParseFloat(string(filter.GetStringBytes("minNotional")), 64)
			}
			nameInExchange = string(symbol.GetStringBytes("symbol"))
			baseAsset = string(symbol.GetStringBytes("baseAsset"))
			quoteAsset = string(symbol.GetStringBytes("quoteAsset"))
			symbolName = model.NewSymbolName(baseAsset, quoteAsset)
		}

		symbolInfo := &model.SymbolInfo{
			NameInExchange:      nameInExchange,
			BaseAsset:           baseAsset,
			BaseAssetInExchange: baseAsset,
			QuoteAsset:          quoteAsset,
			SymbolName:          symbolName,
			InstType:            model.Spot,
			PricePrecision:      pricePrecision,
			QuantityPrecision:   quantityPrecision,
			CtVal:               1,
			CtMult:              1,
			MinValue:            minValue,
		}

		allSymbolInfo.Set(e.name, symbolInfo)
	}

	return
}

/* ========================= HTTP底层执行 ========================= */

func (e *BinanceSpotExchange) sendNone(url string, method string, payload map[string]string, isLog bool) (code int, res []byte, err error) {

	httpClient := e.httpManger.NewClient()
	req := httpClient.Req
	resp := httpClient.Resp
	defer httpClient.Drop()

	if payload != nil {
		url = url + "?" + utils.GetAndEqJionString(payload)
	}
	req.Header.SetMethod(method)
	req.SetRequestURI(url)

	if isLog {
		e.logger.Log(slog.LevelDebug, "HTTP Send", "url", url, "exchange", string(e.name))
	}
	err = httpClient.Client.Do(req, resp)
	if err != nil {
		e.logger.Log(slog.LevelError, "HTTP Error", "url", url, "exchange", string(e.name), "error", err)
		return
	}

	code = resp.StatusCode()
	res = resp.Body()

	if res == nil {
		res = []byte("null")
	}

	if isLog {
		e.logger.Log(slog.LevelDebug, "HTTP Res", "res", string(res), "exchange", string(e.name))
	}

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)

	if weight > e.rateLimitsMinuteCount {
		err = model.ErrIpRateLimits
		e.logger.Log(slog.LevelWarn, "WS Api RateLimits", "error", "MINUTE 1分钟限速超限", "weight", weight, "exchange", string(e.name))
		//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	weight="+strconv.FormatInt(weight, 10)+"	exchange="+string(e.name))
		e.rateLimitC <- err
	}

	return
}
func (e *BinanceSpotExchange) sendSign(url string, method string, payload map[string]string, isLog bool) (code int, res []byte, err error) {
	httpClient := e.httpManger.NewClient()
	req := httpClient.Req
	resp := httpClient.Resp
	defer httpClient.Drop()

	req.Header.SetMethod(method)
	req.Header.Add("X-MBX-APIKEY", e.apiKey)
	req.Header.Add("Content-Type", "application/json;charset=utf-8")

	url = url + "?" + e.addHttpSign(utils.GetAndEqJionString(payload))
	req.SetRequestURI(url)

	if isLog {
		e.logger.Log(slog.LevelDebug, "HTTP Send", "url", url, "exchange", string(e.name))
	}
	err = httpClient.Client.Do(req, resp)
	if err != nil {
		e.logger.Log(slog.LevelError, "HTTP Error", "url", url, "exchange", string(e.name), "error", err)
		return
	}
	if isLog {
		e.logger.Log(slog.LevelDebug, "HTTP Res", "res", string(res), "exchange", string(e.name))
	}

	code = resp.StatusCode()
	res = resp.Body()

	if res == nil {
		res = []byte("null")
	}

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)
	if weight > e.rateLimitsMinuteCount {
		err = model.ErrIpRateLimits
		e.logger.Log(slog.LevelWarn, "WS Api RateLimits", "error", "MINUTE 1分钟限速超限", "weight", weight, "exchange", string(e.name))
		//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	weight="+strconv.FormatInt(weight, 10)+"	exchange="+string(e.name))
		e.rateLimitC <- err
	}

	return
}
func (e *BinanceSpotExchange) sendApik(url string, method string, payload map[string]string, isLog bool) (code int, res []byte, err error) {
	httpClient := e.httpManger.NewClient()
	req := httpClient.Req
	resp := httpClient.Resp
	defer httpClient.Drop()

	req.Header.SetMethod(method)
	req.Header.Add("X-MBX-APIKEY", e.apiKey)
	req.Header.Add("Content-Type", "application/json;charset=utf-8")

	if payload != nil {
		url = url + "?" + utils.GetAndEqJionString(payload)
	}
	req.SetRequestURI(url)

	if isLog {
		e.logger.Log(slog.LevelDebug, "HTTP Send", "url", url, "exchange", string(e.name))
	}
	err = httpClient.Client.Do(req, resp)
	if err != nil {
		e.logger.Log(slog.LevelError, "HTTP Error", "url", url, "exchange", string(e.name), "error", err)
		return
	}
	if isLog {
		e.logger.Log(slog.LevelDebug, "HTTP Res", "res", string(res), "exchange", string(e.name))
	}

	code = resp.StatusCode()
	res = resp.Body()

	if res == nil {
		res = []byte("null")
	}

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)
	if weight > e.rateLimitsMinuteCount {
		err = model.ErrIpRateLimits
		e.logger.Log(slog.LevelWarn, "WS Api RateLimits", "error", "MINUTE 1分钟限速超限", "weight", weight, "exchange", string(e.name))
		//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	weight="+strconv.FormatInt(weight, 10)+"	exchange="+string(e.name))
		e.rateLimitC <- err
	}

	return
}

/* ================================================ 获取实时数据 ================================================ */

func (e *BinanceSpotExchange) GetHTTPServerTime() (timeStamp int64, err error) {
	url := e.getBaseUrl() + "/api/v3/time"
	_, res, err := e.sendNone(url, http.MethodGet, nil, false)
	if err != nil {
		return
	}

	var parser fastjson.Parser

	resData, err := parser.ParseBytes(res)
	if err != nil {
		return
	}

	timeStamp = resData.GetInt64("serverTime")

	return
}
func (e *BinanceSpotExchange) GetHTTPPrice(symbol model.SymbolName) (price float64, err error) {
	url := e.getBaseUrl() + "/api/v3/ticker/price"

	nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(e.name, symbol)
	if !ok {
		e.logger.Log(slog.LevelWarn, "GetHTTPPrice Error", "error", "symbol not found in symbolInfo", "symbol", string(symbol), "exchange", string(e.name))
		return price, errors.New("symbol not found")
	}

	payload := map[string]string{
		"symbol": nameInExchange,
	}
	_, res, err := e.sendNone(url, http.MethodGet, payload, true)
	if err != nil {
		return
	}

	var parser fastjson.Parser

	value, err := parser.ParseBytes(res)
	if err != nil {
		return
	}

	price = utils.StringToFloat64(string(value.GetStringBytes("price")))

	return
}
func (e *BinanceSpotExchange) GetHTTPBookTicker(symbol model.SymbolName) (bookTicker *model.BookTicker, err error) {

	url := e.getBaseUrl() + "/api/v3/ticker/bookTicker"

	nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(e.name, symbol)
	if !ok {
		e.logger.Log(slog.LevelWarn, "GetHTTPBookTicker Error", "error", "symbol not found in symbolInfo", "symbol", string(symbol), "exchange", string(e.name))
		return bookTicker, errors.New("symbol not found")
	}

	payload := map[string]string{
		"symbol": nameInExchange,
	}
	_, res, err := e.sendNone(url, http.MethodGet, payload, true)
	if err != nil {
		return
	}

	var parser fastjson.Parser

	data, err := parser.ParseBytes(res)
	if err != nil {
		return
	}
	askPrice := data.GetFloat64("askPrice")
	askQty := data.GetFloat64("askQty")
	bidPrice := data.GetFloat64("bidPrice")
	bidQty := data.GetFloat64("bidQty")

	bookTicker = model.NewBookTicker(
		symbol,
		askPrice,
		askQty,
		bidPrice,
		bidQty,
		0,
		0,
	)

	return
}
func (e *BinanceSpotExchange) GetHTTPBalance() (balance *model.AllBalance, err error) {
	balance = model.NewAllBalance()

	url := e.getBaseUrl() + "/api/v3/account"

	_, res, err := e.sendSign(url, http.MethodGet, map[string]string{"omitZeroBalances": "true"}, true)
	if err != nil {
		e.logger.Log(slog.LevelWarn, "GetHTTPBalance error", "error", err.Error())
		return
	}

	var parser fastjson.Parser

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Log(slog.LevelWarn, "GetHTTPBalance error", "error", err.Error())
		return
	}

	balances := data.GetArray("balances")
	for i := range balances {
		asset := string(balances[i].GetStringBytes("asset"))
		free := utils.StringToFloat64(string(balances[i].GetStringBytes("free")))
		locked := utils.StringToFloat64(string(balances[i].GetStringBytes("locked")))
		balance.Update(asset, &model.Balance{Free: free, Locked: locked})
	}

	return
}
func (e *BinanceSpotExchange) GetHTTPPosition() (position *model.AllPosition, err error) {
	err = errors.New("spot exchange have not GetHTTPPosition()")
	return
}

/* ================================================ 修改账户设置 ================================================ */

func (e *BinanceSpotExchange) SetPositionSide(isDualMode bool) (err error) {
	err = errors.New("spot exchange have not SetPositionSide()")
	return
}
func (e *BinanceSpotExchange) SetMarginType(isCross bool) (err error) {
	err = errors.New("spot exchange have not SetMarginType()")
	return
}
func (e *BinanceSpotExchange) SetLeverage(symbol model.SymbolName, lev int) (err error) {
	err = errors.New("spot exchange have not SetLeverage()")
	return
}
func (e *BinanceSpotExchange) SetAllLeverage(lev int) (err error) {
	err = errors.New("spot exchange have not SetAllLeverage()")
	return
}

/* ================================================ websocket订阅状态更新数据 ================================================ */

func (e *BinanceSpotExchange) GetRateLimitC() chan error {
	return e.rateLimitC
}
func (e *BinanceSpotExchange) GetStructOfAllBanlance() (allBalance *model.AllBalance) {
	return e.allBalance
}
func (e *BinanceSpotExchange) GetStructOfAllPosition() (allPosition *model.AllPosition) {
	return e.allPosition
}
func (e *BinanceSpotExchange) GetStructOfAllSymbolInfo() (allSymbolInfo *model.AllSymbolsInfo) {
	return e.allSymbolInfo
}
func (e *BinanceSpotExchange) SetSubBalanceCallback(callback func(data *fastjson.Value)) {
	if callback == nil {
		return
	}
	e.balanceCallback = callback
}
func (e *BinanceSpotExchange) SetSubOrderCallback(callback func(data *fastjson.Value)) {
	if callback == nil {
		return
	}
	e.orderCallback = callback
}

/* ================================================ websocket订阅业务驱动数据 ================================================ */

func (e *BinanceSpotExchange) SubBookTicker(symbols []model.SymbolName, callback func(msg []byte), isGoroutine bool) error {

	symbolStrs := make([]string, 0)
	for _, symbol := range symbols {
		nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(e.name, symbol)
		if !ok {
			e.logger.Log(slog.LevelWarn, "SubBookTicker error", "error", "symbol not found in symbolInfo", "symbol", string(symbol), "exchange", string(e.name))
			return errors.New("symbol not found")
		}
		symbolStrs = append(symbolStrs, strings.ToLower(nameInExchange))
	}

	symbolStrss := utils.Arr2Arr(symbolStrs, 400)

	for i := range symbolStrss {

		url := e.subWsbaseUrl + "/stream?streams=" + strings.Join(symbolStrss[i], "@bookTicker/") + "@bookTicker"

		client, err := e.NewWsClient(url, "", callback, isGoroutine)
		if err != nil {
			e.logger.Log(slog.LevelError, "create websocket client error", "url", url, "error", err.Error())
			return err
		}

		e.wsSubBookTickerClients = append(e.wsSubBookTickerClients, client)
	}

	return nil
}

// func (e *BinanceSpotExchange) SubBookTickerFast(connCountPerLocalIP int, symbols []model.SymbolName, callback func(msg []byte), isGoroutine bool) (err error) {
// 	localIPs, err := utils.GetAllLocalIPs()
// 	if err != nil {
// 		e.logger.Log(slog.LevelWarn, "GetAllLocalIPs error", "error", err.Error(), "exchange", string(e.name))
// 		return err
// 	}

// 	url := e.subWsbaseUrl + "/stream?streams="

// 	streams := make([]string, 0)
// 	for _, symbol := range symbols {
// 		nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(e.name, symbol)
// 		if !ok {
// 			e.logger.Log(slog.LevelWarn, "SubBookTicker error", "error", "symbol not found in symbolInfo", "symbol", string(symbol), "exchange", string(e.name))
// 			return errors.New("symbol not found")
// 		}
// 		streams = append(streams, strings.ToLower(nameInExchange)+"@bookTicker")
// 	}

// 	parseUpdateIDFunc := func(msg []byte) (stream string, updateID int64, isScoringMsg bool) {
// 		stream = fastjson.GetString(msg, "stream")
// 		if stream == "btcusdt@bookTicker" {
// 			isScoringMsg = true
// 		}
// 		updateID = int64(fastjson.GetInt(msg, "data", "u"))

// 		return
// 	}

// 	// 1. WebSocket服务器地址。
// 	// 2. 订阅的频道列表。
// 	// 3. 频道是否拼接到url中
// 	// 4. 每个连接的频道数量（防止单个连接订阅的频道数量超过限制）
// 	// 5. 登录并订阅的函数
// 	// 6. 心跳函数
// 	// 7. 心跳时间
// 	// 8. 是否使用协程处理消息
// 	// 9. 本地IP列表
// 	// 10. 每个本地IP的连接数量
// 	// 11. 解析更新ID的函数
// 	// 12. 数据处理回调函数
// 	return jcdws.OpenFastWs(
// 		url,
// 		streams,
// 		true,
// 		400,
// 		nil,
// 		e.wsHeartFunc,
// 		e.wsHeartTime,
// 		isGoroutine,
// 		localIPs,
// 		connCountPerLocalIP,
// 		parseUpdateIDFunc,
// 		callback)
// }

// TODO /* ================================================ 订单操作 ================================================ */

func (e *BinanceSpotExchange) NewOrder(newOrder model.NewOrder) (orderDetail *model.OrderDetail, err error) {
	orderDetail = &model.OrderDetail{}

	var (
		id   = strconv.FormatInt(time.Now().UnixNano(), 10)
		resC = make(chan *fastjson.Value, 5)
	)
	e.operateWsReadChanMap.Store(id, resC)

	// 构建下单参数
	symbolInfo, ok := e.allSymbolInfo.Get(e.name, "", newOrder.Symbol)
	if !ok {
		e.logger.Log(slog.LevelWarn, "NewOrder error", "error", "symbol not found in symbolInfo", "symbol", string(newOrder.Symbol), "exchange", string(e.name))
		return orderDetail, errors.New("symbol not found")
	}

	side := ""
	switch newOrder.Side {
	case model.Buy:
		side = "BUY"
	case model.Sell:
		side = "SELL"
	}

	timeInForce := ""
	switch newOrder.TimeInForce {
	case model.GTC:
		timeInForce = "GTC"
	case model.IOC:
		timeInForce = "IOC"
	case model.FOK:
		timeInForce = "FOK"
	}

	params := map[string]string{
		"symbol":           symbolInfo.NameInExchange,
		"side":             side,
		"newOrderRespType": "RESULT",
		"timestamp":        strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
	}

	if newOrder.ClientOrderID != "" {
		params["newClientOrderId"] = newOrder.ClientOrderID
	}

	if newOrder.QuoteOrderQty != 0 {
		params["quoteOrderQty"] = utils.Float64ToString(utils.FormatFloat(newOrder.QuoteOrderQty, 0))
	} else {
		qty := newOrder.Quantity / symbolInfo.CtMult / symbolInfo.CtVal
		qty = utils.FormatFloat(qty, symbolInfo.QuantityPrecision)
		params["quantity"] = utils.Float64ToString(qty)
	}

	switch newOrder.OType {
	case model.Limit:
		params["type"] = "LIMIT"
		params["price"] = utils.Float64ToString(utils.FormatFloat(newOrder.Price, symbolInfo.PricePrecision))
		params["timeInForce"] = timeInForce
	case model.Market:
		params["type"] = "MARKET"
	}

	sendMsg := map[string]any{
		"id":     id,
		"method": "order.place",
		"params": params,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return
	}

	// 下单
	err = e.wsOrderClient.SendMessage(payload)
	if err != nil {
		return
	}

	// 接收返回
	data := <-resC

	orderDetail.ClientOrderID = newOrder.ClientOrderID
	orderDetail.Symbol = newOrder.Symbol
	orderDetail.Side = newOrder.Side
	orderDetail.OType = newOrder.OType
	orderDetail.OrigQty = newOrder.Quantity
	orderDetail.OrigPrice = newOrder.Price

	switch string(data.GetStringBytes("result", "status")) {
	case "NEW":
		orderDetail.State = model.New
	case "PARTIALLY_FILLED":
		orderDetail.State = model.Partially_Filled
	case "FILLED":
		orderDetail.State = model.Filled
	case "CANCELED":
		orderDetail.State = model.Canceled
	case "EXPIRED":
		orderDetail.State = model.Expired
	default:
		orderDetail.ErrCode = data.GetInt("error", "code")
		orderDetail.ErrMsg = string(data.GetStringBytes("error", "msg"))
		orderDetail.State = model.Fail
		return
	}

	executedQty := utils.StringToFloat64(string(data.GetStringBytes("result", "executedQty")))
	executedMoney := utils.StringToFloat64(string(data.GetStringBytes("result", "cummulativeQuoteQty")))
	orderDetail.ExecutedQty = executedQty
	orderDetail.ExecutedMoney = executedMoney
	if executedQty != 0 {
		orderDetail.ExecutedAvPrice = executedMoney / executedQty
	}
	return
}

func (e *BinanceSpotExchange) GetOrder(symbol model.SymbolName, clientOrderID string) (orderDetail *model.OrderDetail, err error) {
	orderDetail = &model.OrderDetail{}

	var (
		id   = strconv.FormatInt(time.Now().UnixNano(), 10)
		resC = make(chan *fastjson.Value, 5)
	)
	e.operateWsReadChanMap.Store(id, resC)

	sendMsg := map[string]any{
		"id": id,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return
	}

	// 下单
	err = e.wsOrderClient.SendMessage(payload)
	if err != nil {
		return
	}

	msg := <-resC
	_ = msg

	return
}
func (e *BinanceSpotExchange) DelOrder(symbol model.SymbolName, clientOrderID string) (err error) {

	var (
		resC = make(chan *fastjson.Value)
		id   = utils.GetUUID()
	)
	e.operateWsReadChanMap.Store(id, resC)

	sendMsg := map[string]any{
		"id": id,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return
	}

	// 下单
	err = e.wsOrderClient.SendMessage(payload)
	if err != nil {
		return
	}

	msg := <-resC
	_ = msg

	return
}
func (e *BinanceSpotExchange) DelAllOrder(symbol model.SymbolName) (err error) {

	var (
		resC = make(chan *fastjson.Value)
		id   = utils.GetUUID()
	)
	e.operateWsReadChanMap.Store(id, resC)

	sendMsg := map[string]any{
		"id": id,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return
	}

	// 下单
	err = e.wsOrderClient.SendMessage(payload)
	if err != nil {
		return
	}

	msg := <-resC
	_ = msg

	return
}
func (e *BinanceSpotExchange) DelReplace(newOrder model.NewOrder, cancelClientOrderID string) (orderDetail *model.OrderDetail, err error) {
	orderDetail = &model.OrderDetail{}

	var (
		resC = make(chan *fastjson.Value)
		id   = utils.GetUUID()
	)
	e.operateWsReadChanMap.Store(id, resC)

	sendMsg := map[string]any{
		"id": id,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return
	}

	// 下单
	err = e.wsOrderClient.SendMessage(payload)
	if err != nil {
		return
	}

	msg := <-resC
	_ = msg

	return
}

func (e *BinanceSpotExchange) Test() {
	url := e.getBaseUrl() + "/sapi/v1/sub-account/subAccountApi/ipRestriction"
	e.sendNone(url, http.MethodGet, map[string]string{
		"email":            "624_virtual@cqmvhds0managedsub.com",
		"subAccountApiKey": "L5uh9sCbrThUpiRrrYOSEhOp1lgzLG8G4S9SFDbuSPZfIGRumplPlrRG9ElrPLnM",
		"timestamp":        strconv.FormatInt(time.Now().UnixMilli(), 10),
	}, true)
}

var _ exchange.Exchange = (*BinanceSpotExchange)(nil)
