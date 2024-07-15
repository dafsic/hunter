package perps

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/exchange/binance"
	"github.com/dafsic/hunter/exchange/model"
	jcdhttp "github.com/dafsic/hunter/pkg/http"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"
	"github.com/dafsic/hunter/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
)

type BinancePerpsExchange struct {
	apiKey            string
	secretKey         string
	ed25519ApiKey     string
	ed25519PrivateKey ed25519.PrivateKey
	name              model.ExchangeName // 交易所名称

	allBalance    *model.AllBalance     // 账户信息
	allPosition   *model.AllPosition    // 持仓信息
	allSymbolInfo *model.AllSymbolsInfo // 交易规范

	balanceCallback  func(data *fastjson.Value) // 账户更新回调函数
	positionCallback func(data *fastjson.Value) // 持仓更新回调函数
	orderCallback    func(data *fastjson.Value) // 订单更新回调函数

	wsOrderClient          *binance.WsClient     // 下单专用通道
	wsApiClient            *binance.WsClient     // wsApi发送通道
	wsAccountClient        *binance.WsClient     // 账户通道
	wsSubBookTickerClients []*binance.WsClient   // 订阅最优挂单通道数组
	operateWsReadChanMap   *sync.Map             // wsApi广播Map  string:chan *fastjson.Value
	orderDetailMap         *model.OrderDetailMap // 订单映射 (ClientOrderID:*model.OrderDetail)

	rateLimitsMinuteCount   int64      // IP限速报警阈值
	rateLimitsDayCount      int64      // 下单1天限速报警阈值
	rateLimitsSecond10Count int64      // 下单10秒限速报警阈值
	rateLimitC              chan error // 限速警报通道

	operateWsUrl     string        // wsApi wsUrl
	subWsbaseUrl     string        // 订阅型wsUrl
	wsHeartBeat      int           // 心跳时间
	baseHTTPUrl      string        // baseUrl
	baseHTTPUrlS     []string      // baseUrl合集
	baseHTTPUrlRlock *sync.RWMutex // baseUrl读写锁

	wsManager  ws.Manager         // ws管理器
	httpManger jcdhttp.Manager    // http client 管理器
	logger     log.Logger         // 日志
	wg         sync.WaitGroup     // 等待组
	ctx        context.Context    // 上下文
	cancelFunc context.CancelFunc // 用于交易所退出时，通知所有goroutine退出
}

/* ========================= 构建和初始化 ========================= */

func NewBinancePerps(l log.Logger, wsMgr ws.Manager, httpMgr jcdhttp.Manager, cfg config.BinanceCfg) (*BinancePerpsExchange, error) {
	exchange := &BinancePerpsExchange{
		apiKey:        cfg.ApiKey,
		secretKey:     cfg.SecretKey,
		ed25519ApiKey: cfg.Ed25519ApiKey,
		name:          model.BinancePerps,

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

		operateWsUrl:     "wss://ws-fapi.binance.com/ws-fapi/v1",
		subWsbaseUrl:     "wss://fstream.binance.com",
		wsHeartBeat:      60,
		baseHTTPUrl:      "https://fapi.binance.com",
		baseHTTPUrlRlock: new(sync.RWMutex),
		baseHTTPUrlS:     []string{},

		wsManager:  wsMgr,
		httpManger: httpMgr,
		logger:     l,
	}

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

func (e *BinancePerpsExchange) Init() (err error) {
	e.logger.Info("binanceSpot exchange init...")
	defer e.logger.Info("binanceSpot exchange init done")

	// 初始化交易规范
	e.allSymbolInfo, err = e.getSymbolInfo()
	if err != nil {
		return fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	if e.apiKey == "" {
		return
	}

	// 建立下单专用连接
	e.wsOrderClient, err = binance.NewWsClientWithReconnect(e.logger, e.wsManager, e.operateWsUrl, "", e.operateWsCallback, e.wsHeartBeat, false)
	if err != nil {
		e.logger.Error("create websocket client error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}
	err = e.wsLogin(e.wsOrderClient)
	if err != nil {
		e.logger.Error("websocket login error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}

	// 建立操作ws连接
	e.wsApiClient, err = binance.NewWsClientWithReconnect(e.logger, e.wsManager, e.operateWsUrl, "", e.operateWsCallback, e.wsHeartBeat, false)
	if err != nil {
		e.logger.Error("create websocket client error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}
	err = e.wsLogin(e.wsOrderClient)
	if err != nil {
		e.logger.Error("websocket login error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}

	// 初始化账户信息
	e.allBalance, err = e.GetHTTPBalance()
	if err != nil {
		e.logger.Error("GetHTTPBalance fail", "error", err)
		return err
	}

	// 为什么原来没有listenkey？？？？
	e.wsAccountClient, err = binance.NewWsClientWithReconnect(e.logger, e.wsManager, e.subWsbaseUrl+"/ws/"+e.getLisenKey(), "", e.accountWsCallback, e.wsHeartBeat, true)
	if err != nil {
		e.logger.Error("create websocket client error", "url", e.operateWsUrl, "error", err.Error())
		return err
	}

	// 更新交易规范
	go e.updateAllSymbolInfo()

	return
}

func (e *BinancePerpsExchange) Exit() {
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
	e.logger.Info("exchange exit", "exchange", e.name)
	e.logger.Flush()
}

func (e *BinancePerpsExchange) updateAllSymbolInfo() {
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

func (e *BinancePerpsExchange) wsLogin(client *binance.WsClient) error {
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

/* ========================= 辅助函数 ========================= */
func (e *BinancePerpsExchange) getBaseUrl() string {
	// 获取现货baseUrl
	e.baseHTTPUrlRlock.RLock()
	defer e.baseHTTPUrlRlock.RUnlock()
	return e.baseHTTPUrl
}

func (e *BinancePerpsExchange) addHttpSign(urlPath string) string {
	// HTTP鉴权请求添加 timestamp、recvWindow、signature 参数
	if urlPath != "" {
		urlPath += "&"
	}
	urlPath += "timestamp=" + strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	urlPath += "&recvWindow=3500"
	urlPath += "&signature=" + utils.GetHamcSha256HexEncodeSignStr(urlPath, e.secretKey)
	return urlPath
}

func (e *BinancePerpsExchange) operateWsCallback(msg []byte) {
	// websocket API 的回调函数
	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		e.logger.Warn("parse ws message error", "error", err.Error())
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
				e.logger.Warn("websocket api rate limits", "error", "SECOND 10秒限速超限", "count", count)
				//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=SECOND 10s限速超限	count="+strconv.FormatInt(count, 10)+"	exchange="+string(e.name))
			}
		case "DAY":
			if count > e.rateLimitsSecond10Count {
				e.rateLimitC <- model.ErrOrderRateLimits
				e.logger.Warn("websocket api rate limits", "error", "DAY 1天限速超限", "count", count)
				//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=DAY 1天限速超限	count="+strconv.FormatInt(count, 10)+"	exchange="+string(e.name))
			}
		case "MINUTE":
			if count > e.rateLimitsSecond10Count {
				e.rateLimitC <- model.ErrIpRateLimits
				e.logger.Warn("websocket api rate limits", "error", "MITNUTE 1分钟限速超限", "count", count)
				//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	count="+strconv.FormatInt(count, 10)+"	exchange="+string(e.name))
			}
		}
	}

	e.logger.Info("websocket api event", "res", string(msg))
}

func (e *BinancePerpsExchange) accountWsCallback(msg []byte) {
	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		e.logger.Warn("parse ws message error", "error", err.Error())
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

	e.logger.Info("websocket account event", "res", string(msg))
}

func (e *BinancePerpsExchange) getLisenKey() (lisenKey string) {
	// 获取现货LisenKey
	urlPath := e.getBaseUrl() + "fapi/v1/listenKey"
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

func (e *BinancePerpsExchange) lisenKeyHearbeat(lisenKey string) {
	// 更新 LisenKey 有效期
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
			urlPath := e.getBaseUrl() + "/fapi/v1/listenKey"
			e.sendApik(urlPath, "PUT", map[string]string{"listenKey": lisenKey}, false)
		}
	}
}

func (e *BinancePerpsExchange) getSymbolInfo() (allSymbolInfo *model.AllSymbolsInfo, err error) {
	var (
		parser  fastjson.Parser
		urlPath = e.getBaseUrl() + "/fapi/v1/exchangeInfo"
		res     []byte
	)

	_, res, err = e.sendNone(urlPath, "GET", nil, false)
	if err != nil {
		e.logger.Warn("getSymbolInfo error", "error", err)
		return
	}

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Warn("parse ws message error", "error", err.Error())
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

func (e *BinancePerpsExchange) sendNone(url string, method string, payload map[string]string, isLog bool) (code int, res []byte, err error) {

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
		e.logger.Debug("HTTP Send", "url", url)
	}
	err = httpClient.Client.Do(req, resp)
	if err != nil {
		e.logger.Error("HTTP Error", "url", url, "error", err)
	}

	code = resp.StatusCode()
	res = resp.Body()

	if isLog {
		e.logger.Debug("HTTP Res", "res", string(res))
	}

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)

	if weight > e.rateLimitsMinuteCount {
		err = model.ErrIpRateLimits
		e.logger.Warn("WS Api RateLimits", "error", "MINUTE 1分钟限速超限", "weight", weight)
		//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	weight="+strconv.FormatInt(weight, 10)+"	exchange="+string(e.name))
		e.rateLimitC <- err
	}

	return
}

func (e *BinancePerpsExchange) sendSign(url string, method string, payload map[string]string, isLog bool) (code int, res []byte, err error) {
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
		e.logger.Debug("HTTP Send", "url", url)
	}
	err = httpClient.Client.Do(req, resp)
	if err != nil {
		e.logger.Error("HTTP Error", "url", url, "error", err)
	}
	if isLog {
		e.logger.Debug("HTTP Res", "res", string(res))
	}

	code = resp.StatusCode()
	res = resp.Body()

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)
	if weight > e.rateLimitsMinuteCount {
		err = model.ErrIpRateLimits
		e.logger.Warn("WS Api RateLimits", "error", "MINUTE 1分钟限速超限", "weight", weight)
		//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	weight="+strconv.FormatInt(weight, 10)+"	exchange="+string(e.name))
		e.rateLimitC <- err
	}

	return
}
func (e *BinancePerpsExchange) sendApik(url string, method string, payload map[string]string, isLog bool) (code int, res []byte, err error) {
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
		e.logger.Debug("HTTP Send", "url", url)
	}
	err = httpClient.Client.Do(req, resp)
	if err != nil {
		e.logger.Error("HTTP Error", "url", url, "error", err)
	}
	if isLog {
		e.logger.Debug("HTTP Res", "res", string(res))
	}

	code = resp.StatusCode()
	res = resp.Body()

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)
	if weight > e.rateLimitsMinuteCount {
		err = model.ErrIpRateLimits
		e.logger.Warn("WS Api RateLimits", "error", "MINUTE 1分钟限速超限", "weight", weight)
		//jcdlog.TgString(model.Warning, "msg=WS Api RateLimits	error=MINUTE 1分钟限速超限	weight="+strconv.FormatInt(weight, 10)+"	exchange="+string(e.name))
		e.rateLimitC <- err
	}

	return
}

/* ================================================ 获取实时数据 ================================================ */

func (e *BinancePerpsExchange) GetHTTPServerTime() (timeStamp int64, err error) {
	url := e.getBaseUrl() + "/fapi/v1/time"
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

func (e *BinancePerpsExchange) GetHTTPPrice(symbol model.SymbolName) (price float64, err error) {
	url := e.getBaseUrl() + "/fapi/v1/ticker/price"

	symbolInterface, ok := e.allSymbolInfo.Get(e.name, "", symbol)
	if !ok {
		e.logger.Warn("GetHTTPPrice Error", "error", "symbol not found in symbolInfo", "symbol", string(symbol))
		return price, errors.New("symbol not found")
	}

	payload := map[string]string{
		"symbol": symbolInterface.NameInExchange,
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

func (e *BinancePerpsExchange) GetHTTPBookTicker(symbol model.SymbolName) (bookTicker *model.BookTicker, err error) {
	bookTicker = &model.BookTicker{}

	url := e.getBaseUrl() + "/fapi/v1/ticker/bookTicker"

	symbolInterface, ok := e.allSymbolInfo.Get(e.name, "", symbol)
	if !ok {
		e.logger.Warn("GetHTTPBookTicker Error", "error", "symbol not found in symbolInfo", "symbol", string(symbol))
		return bookTicker, errors.New("symbol not found")
	}

	payload := map[string]string{
		"symbol": symbolInterface.NameInExchange,
	}
	_, res, err := e.sendNone(url, http.MethodGet, payload, true)
	if err != nil {
		return
	}

	var parser fastjson.Parser

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Warn("parse ws message error", "error", err.Error())
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

func (e *BinancePerpsExchange) GetHTTPBalance() (balance *model.AllBalance, err error) {
	balance = model.NewAllBalance()

	url := e.getBaseUrl() + "/fapi/v2/account"

	_, res, err := e.sendSign(url, http.MethodGet, map[string]string{"omitZeroBalances": "true"}, true)
	if err != nil {
		e.logger.Warn("GetHTTPBalance error", "error", err.Error())
		return
	}

	var parser fastjson.Parser

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Warn("parse ws message error", "error", err.Error())
		return
	}

	balances := data.GetArray("assets")
	for _, v := range balances {
		asset := string(v.GetStringBytes("asset"))
		free := v.GetFloat64("maxWithdrawAmount")
		//locked, _ := v.Get("locked").Float64()
		balance.Update(asset, &model.Balance{Free: free})
	}

	return
}
func (e *BinancePerpsExchange) GetHTTPPosition() (position *model.AllPosition, err error) {
	err = errors.New("spot exchange have not GetHTTPPosition()")
	return
}

/* ================================================ 修改账户设置 ================================================ */

func (e *BinancePerpsExchange) SetPositionSide(isDualMode bool) (err error) {
	err = errors.New("spot exchange have not SetPositionSide()")
	return
}
func (e *BinancePerpsExchange) SetMarginType(isCross bool) (err error) {
	err = errors.New("spot exchange have not SetMarginType()")
	return
}
func (e *BinancePerpsExchange) SetLeverage(symbol model.SymbolName, lev int) (err error) {
	err = errors.New("spot exchange have not SetLeverage()")
	return
}
func (e *BinancePerpsExchange) SetAllLeverage(lev int) (err error) {
	err = errors.New("spot exchange have not SetAllLeverage()")
	return
}

/* ================================================ websocket订阅状态更新数据 ================================================ */

func (e *BinancePerpsExchange) GetRateLimitC() chan error {
	return e.rateLimitC
}
func (e *BinancePerpsExchange) GetStructOfAllBanlance() (allBalance *model.AllBalance) {
	return e.allBalance
}
func (e *BinancePerpsExchange) GetStructOfAllPosition() (allPosition *model.AllPosition) {
	return e.allPosition
}
func (e *BinancePerpsExchange) GetStructOfAllSymbolInfo() (allSymbolInfo *model.AllSymbolsInfo) {
	return e.allSymbolInfo
}
func (e *BinancePerpsExchange) SetSubBalanceCallback(callback func(data *fastjson.Value)) {
	if callback == nil {
		return
	}
	e.balanceCallback = callback
}
func (e *BinancePerpsExchange) SetSubOrderCallback(callback func(data *fastjson.Value)) {
	if callback == nil {
		return
	}
	e.orderCallback = callback
}

/* ================================================ websocket订阅业务驱动数据 ================================================ */

func (e *BinancePerpsExchange) SubBookTicker(symbols []model.SymbolName, callback func(msg []byte), isGoroutine bool) error {

	symbolStrs := make([]string, 0)
	for _, symbol := range symbols {
		symbolInterface, ok := e.allSymbolInfo.Get(e.name, "", symbol)
		if !ok {
			e.logger.Warn("SubBookTicker error", "error", "symbol not found in symbolInfo", "symbol", string(symbol))
			return errors.New("symbol not found")
		}
		symbolStrs = append(symbolStrs, strings.ToLower(symbolInterface.NameInExchange))
	}

	symbolStrss := utils.Arr2Arr(symbolStrs, 200)

	for i := range symbolStrss {
		url := e.subWsbaseUrl + "/stream?streams=" + strings.Join(symbolStrss[i], "@bookTicker/") + "@bookTicker"

		client, err := binance.NewWsClientWithReconnect(e.logger, e.wsManager, url, "", callback, e.wsHeartBeat, isGoroutine)
		if err != nil {
			e.logger.Error("create websocket client error", "url", url, "error", err.Error())
			return err
		}

		e.wsSubBookTickerClients = append(e.wsSubBookTickerClients, client)
	}

	return nil
}

// func (e *BinancePerpsExchange) SubBookTickerFast(connCountPerLocalIP int, symbols []model.SymbolName, callback func(msg []byte), isGoroutine bool) (err error) {
// 	localIPs, err := utils.GetAllLocalIPs()
// 	if err != nil {
// 		jcdlog.LogString(model.Warning, "GetAllLocalIPs GetAllLocalIPs error="+
// 			err.Error()+"	exchange="+string(e.name))
// 		return err
// 	}

// 	url := e.subWsbaseUrl + "/stream?streams="

// 	streams := make([]string, 0)
// 	for _, symbol := range symbols {
// 		nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(e.name, symbol)
// 		if !ok {
// 			jcdlog.LogString(model.Warning, "msg=SubBookTicker Error"+
// 				"	error=symbol not found in symbolInfo"+
// 				"	symbol="+string(symbol)+
// 				"	exchange="+string(e.name))
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

func (e *BinancePerpsExchange) NewOrder(newOrder model.NewOrder) (orderDetail *model.OrderDetail, err error) {
	orderDetail = &model.OrderDetail{}

	var (
		id   = strconv.FormatInt(time.Now().UnixNano(), 10)
		resC = make(chan *fastjson.Value, 5)
	)
	e.operateWsReadChanMap.Store(id, resC)

	// 构建下单参数
	symbol, ok := e.allSymbolInfo.Get(e.name, "", newOrder.Symbol)
	if !ok {
		e.logger.Warn("NewOrder error", "error", "symbol not found in symbolInfo", "symbol", string(newOrder.Symbol))
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
		"symbol":           symbol.NameInExchange,
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
		params["quantity"] = utils.Float64ToString(utils.FormatFloat(newOrder.Quantity, symbol.QuantityPrecision))
	}

	switch newOrder.OType {
	case model.Limit:
		params["type"] = "LIMIT"
		params["price"] = utils.Float64ToString(utils.FormatFloat(newOrder.Price, symbol.PricePrecision))
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
func (e *BinancePerpsExchange) GetOrder(symbol model.SymbolName, clientOrderID string) (orderDetail *model.OrderDetail, err error) {
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
func (e *BinancePerpsExchange) DelOrder(symbol model.SymbolName, clientOrderID string) (err error) {

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
func (e *BinancePerpsExchange) DelAllOrder(symbol model.SymbolName) (err error) {

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
func (e *BinancePerpsExchange) DelReplace(newOrder model.NewOrder, cancelClientOrderID string) (orderDetail *model.OrderDetail, err error) {
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

func (e BinancePerpsExchange) Test() {
	url := e.getBaseUrl() + "/sapi/v1/sub-account/subAccountApi/ipRestriction"
	e.sendNone(url, http.MethodGet, map[string]string{
		"email":            "624_virtual@cqmvhds0managedsub.com",
		"subAccountApiKey": "L5uh9sCbrThUpiRrrYOSEhOp1lgzLG8G4S9SFDbuSPZfIGRumplPlrRG9ElrPLnM",
		"timestamp":        strconv.FormatInt(time.Now().UnixMilli(), 10),
	}, true)
}

var _ exchange.Exchange = (*BinancePerpsExchange)(nil)
