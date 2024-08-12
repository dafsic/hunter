package perps

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dafsic/hunter/exchange"
	jcdhttp "github.com/dafsic/hunter/pkg/http"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"

	"github.com/dafsic/hunter/utils"

	"github.com/valyala/fastjson"
)

type BinancePerpsExchange struct {
	name             exchange.ExchangeName    // 交易所名称
	allSymbolInfo    *exchange.AllSymbolsInfo // 交易规范
	weightLimit_1m   int64                    // 1分钟内权重限制
	orderLimit_10s   int64                    // 10秒内下单限制
	orderLimit_1d    int64                    // 1天内下单限制
	rateLimitC       chan *exchange.RateLimit // 速率限制信号通道
	operateWsUrl     string                   // wsApi wsUrl
	subWsbaseUrl     string                   // 订阅型wsUrl
	wsHeartBeat      int                      // 心跳时间
	baseHTTPUrl      string                   // baseUrl
	baseHTTPUrlS     []string                 // baseUrl合集
	baseHTTPUrlRlock *sync.RWMutex            // baseUrl读写锁
	websocketManager ws.Manager               // ws管理器
	httpManger       jcdhttp.Manager          // http client 管理器
	logger           log.Logger               // 日志
	wg               sync.WaitGroup           // 等待组
	ctx              context.Context          // 上下文
	cancelFunc       context.CancelFunc       // 用于交易所退出时，通知所有goroutine退出
}

/* ========================= 构建和初始化 ========================= */

func NewBinancePerps(l log.Logger, wsMgr ws.Manager, httpMgr jcdhttp.Manager) (*BinancePerpsExchange, error) {
	exchange := &BinancePerpsExchange{
		name: exchange.BinancePerps,

		allSymbolInfo: exchange.NewAllSymbolsInfo(),

		weightLimit_1m: 5900,
		orderLimit_1d:  19900,
		orderLimit_10s: 95,
		rateLimitC:     make(chan *exchange.RateLimit, 16),

		operateWsUrl:     "wss://ws-fapi.binance.com/ws-fapi/v1",
		subWsbaseUrl:     "wss://fstream.binance.com",
		wsHeartBeat:      60,
		baseHTTPUrl:      "https://fapi.binance.com",
		baseHTTPUrlRlock: new(sync.RWMutex),
		baseHTTPUrlS:     []string{},

		websocketManager: wsMgr,
		httpManger:       httpMgr,
		logger:           l,
	}
	exchange.ctx, exchange.cancelFunc = context.WithCancel(context.Background())

	return exchange, nil
}

func (e *BinancePerpsExchange) Init() (err error) {
	e.logger.Info("binanceSpot exchange init...")
	defer e.logger.Info("binanceSpot exchange init done")

	// 初始化交易规范
	e.allSymbolInfo, err = e.getAllSymbolInfo()
	if err != nil {
		return fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	// 更新最快api
	go e.updateFasterApi()
	// 同步交易规范
	go e.updateAllSymbolInfo()

	// // 建立下单专用连接
	// e.wsOrderClient, err = binance.NewWsClientWithReconnect(e.logger, e.wsManager, e.operateWsUrl, "", e.operateWsCallback, e.wsHeartBeat, false)
	// if err != nil {
	// 	e.logger.Error("create websocket client error", "url", e.operateWsUrl, "error", err.Error())
	// 	return err
	// }
	// err = e.wsLogin(e.wsOrderClient)
	// if err != nil {
	// 	e.logger.Error("websocket login error", "url", e.operateWsUrl, "error", err.Error())
	// 	return err
	// }

	// // 建立操作ws连接
	// e.wsApiClient, err = binance.NewWsClientWithReconnect(e.logger, e.wsManager, e.operateWsUrl, "", e.operateWsCallback, e.wsHeartBeat, false)
	// if err != nil {
	// 	e.logger.Error("create websocket client error", "url", e.operateWsUrl, "error", err.Error())
	// 	return err
	// }
	// err = e.wsLogin(e.wsOrderClient)
	// if err != nil {
	// 	e.logger.Error("websocket login error", "url", e.operateWsUrl, "error", err.Error())
	// 	return err
	// }

	// // 初始化账户信息
	// e.allBalance, err = e.GetHTTPBalance()
	// if err != nil {
	// 	e.logger.Error("GetHTTPBalance fail", "error", err)
	// 	return err
	// }

	// // 为什么原来没有listenkey？？？？
	// e.wsAccountClient, err = binance.NewWsClientWithReconnect(e.logger, e.wsManager, e.subWsbaseUrl+"/ws/"+e.getLisenKey(), "", e.accountWsCallback, e.wsHeartBeat, true)
	// if err != nil {
	// 	e.logger.Error("create websocket client error", "url", e.operateWsUrl, "error", err.Error())
	// 	return err
	// }

	return
}

// CreateOrderController 创建订单控制器
// Parameters:
//   - localIP: 本地IP
//   - apiKey: ed25519 API Key
//   - secretKey: ed25519 Secret Key
//
// Returns:
//   - OrderController: 订单控制器接口
func (e *BinancePerpsExchange) CreateOrderController(localIP, apiKey string, secretKey ed25519.PrivateKey) (exchange.OrderController, error) {
	return NewPerpsOrderController(e, e.operateWsUrl, localIP, apiKey, secretKey, e.wsHeartBeat)
}

func (e *BinancePerpsExchange) Logger() log.Logger {
	return e.logger
}

func (e *BinancePerpsExchange) WebSocketManager() ws.Manager {
	return e.websocketManager
}

func (e *BinancePerpsExchange) CreateMarketController(symbols []exchange.SymbolName, localIP string) (exchange.MarketController, error) {
	if len(symbols) == 0 || len(symbols) > 400 {
		return nil, errors.New("symbols length must be between 1 and 400")
	}

	symbolStrs := make([]string, 0)
	for _, symbol := range symbols {
		symbolInfo := e.allSymbolInfo.Get("", symbol)
		if symbolInfo == nil {
			return nil, errors.New("symbol:" + string(symbol) + " not found")
		}
		symbolStrs = append(symbolStrs, strings.ToLower(symbolInfo.NameInExchange))
	}

	url := e.subWsbaseUrl + "/stream?streams=" + strings.Join(symbolStrs, "@bookTicker/") + "@bookTicker"

	return NewPerpsMarketController(e, url, localIP, e.wsHeartBeat)
}

// CreateAccountController 创建账户控制器
// Parameters:
//   - localIP: 本地IP
func (e *BinancePerpsExchange) CreateAccountController(localIP, apiKey string) (exchange.AccountController, error) {
	return NewPerpsAccountController(e, e.subWsbaseUrl, apiKey, localIP, e.wsHeartBeat)
}

func (e *BinancePerpsExchange) Exit() {
	e.cancelFunc()
	e.wg.Wait()
	e.logger.Info("exchange exit", "exchange", e.name)
	e.logger.Flush()
}

func (e *BinancePerpsExchange) GetSymbolInfo(s exchange.SymbolName) *exchange.SymbolInfo {
	return e.allSymbolInfo.Get("", s)
}

func (e *BinancePerpsExchange) GetSymbloInfoByExchangeName(name string) *exchange.SymbolInfo {
	return e.allSymbolInfo.Get(name, "")
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
			info, err := e.getAllSymbolInfo()
			if err != nil {
				continue
			}
			e.allSymbolInfo.Update(info)
		}
	}
}

func (e *BinancePerpsExchange) updateFasterApi() {
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
				e.Send(urlPath, "GET", nil, nil)
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

/* ========================= 辅助函数 ========================= */
func (e *BinancePerpsExchange) GetBaseUrl() string {
	// 获取现货baseUrl
	e.baseHTTPUrlRlock.RLock()
	defer e.baseHTTPUrlRlock.RUnlock()
	return e.baseHTTPUrl
}

/* ========================= HTTP底层执行 ========================= */
func (e *BinancePerpsExchange) Send(url string, method string, payload map[string]string, header map[string]string) (code int, res []byte, err error) {
	httpClient := e.httpManger.NewClient()
	defer httpClient.Drop()
	req := httpClient.Req
	resp := httpClient.Resp

	if payload != nil {
		url = url + "?" + utils.GetAndEqJionString(payload)
	}
	req.Header.SetMethod(method)
	for k, v := range header {
		req.Header.Add(k, v)
	}
	req.SetRequestURI(url)

	err = httpClient.Do()
	if err != nil {
		return 0, nil, fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	code = resp.StatusCode()
	res = httpClient.Resp.Body()
	if res == nil {
		res = []byte("null")
	}

	weight, _ := strconv.ParseInt(string(resp.Header.Peek("X-Mbx-Used-Weight-1m")), 10, 64)
	if weight > e.weightLimit_1m {
		e.rateLimitC <- exchange.NewRateLimit("error_ip_rate_limits", 60)
	}

	return
}

func (e *BinancePerpsExchange) SendWithSign(url string, method string, payload map[string]string, apiKey, secretKey string) (code int, res []byte, err error) {
	// HTTP鉴权请求添加 timestamp、recvWindow、signature 参数
	payload["timestamp"] = strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	payload["recvWindow"] = "3500"
	payload["signature"] = utils.GetHamcSha256HexEncodeSignStr(utils.GetAndEqJionString(payload), secretKey)

	header := map[string]string{
		"X-MBX-APIKEY": apiKey,
		"Content-Type": "application/json;charset=utf-8",
	}

	return e.Send(url, method, payload, header)
}

func (e *BinancePerpsExchange) SendWithApikey(url string, method string, payload map[string]string, apiKey string) (code int, res []byte, err error) {
	header := map[string]string{
		"X-MBX-APIKEY": apiKey,
		"Content-Type": "application/json;charset=utf-8",
	}

	return e.Send(url, method, payload, header)
}

/* ================================================ 获取实时数据 ================================================ */

func (e *BinancePerpsExchange) GetServerTime() (timeStamp int64, err error) {
	url := e.GetBaseUrl() + "/fapi/v1/time"
	_, res, err := e.Send(url, http.MethodGet, nil, nil)
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

func (e *BinancePerpsExchange) GetPrice(symbol exchange.SymbolName) (price float64, err error) {
	url := e.GetBaseUrl() + "/fapi/v1/ticker/price"

	nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(symbol)
	if !ok {
		e.logger.Warn("GetHTTPPrice Error", "error", "symbol not found in symbolInfo", "symbol", string(symbol))
		return price, errors.New("symbol not found")
	}

	payload := map[string]string{
		"symbol": nameInExchange,
	}

	e.logger.Debug("HTTP Send", "url", url)
	_, res, err := e.Send(url, http.MethodGet, payload, nil)
	if err != nil {
		return
	}
	e.logger.Debug("HTTP Res", "res", string(res))

	var parser fastjson.Parser

	value, err := parser.ParseBytes(res)
	if err != nil {
		return
	}

	price = utils.StringToFloat64(string(value.GetStringBytes("price")))

	return
}

func (e *BinancePerpsExchange) GetBookTicker(symbol exchange.SymbolName) (bookTicker *exchange.BookTicker, err error) {
	bookTicker = &exchange.BookTicker{}

	url := e.GetBaseUrl() + "/fapi/v1/ticker/bookTicker"

	nameInExchange, ok := e.allSymbolInfo.GetNameInExchange(symbol)
	if !ok {
		e.logger.Warn("GetHTTPBookTicker Error", "error", "symbol not found in symbolInfo", "symbol", string(symbol))
		return bookTicker, errors.New("symbol not found")
	}

	payload := map[string]string{
		"symbol": nameInExchange,
	}
	e.logger.Debug("HTTP Send", "url", url)
	_, res, err := e.Send(url, http.MethodGet, payload, nil)
	if err != nil {
		return
	}
	e.logger.Debug("HTTP Res", "res", string(res))

	var parser fastjson.Parser

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Warn("GetHTTPBalance error", "error", err.Error())
		return
	}

	askPrice := data.GetFloat64("askPrice")
	askQty := data.GetFloat64("askQty")
	bidPrice := data.GetFloat64("bidPrice")
	bidQty := data.GetFloat64("bidQty")

	bookTicker = exchange.NewBookTicker(
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

func (e *BinancePerpsExchange) GetBalance(apiKey, secretKey string) (balance *exchange.AllBalance, err error) {
	balance = exchange.NewAllBalance()

	url := e.GetBaseUrl() + "/fapi/v2/account"

	e.logger.Debug("HTTP Send", "url", url)
	_, res, err := e.SendWithSign(url, http.MethodGet, map[string]string{"omitZeroBalances": "true"}, apiKey, secretKey)
	if err != nil {
		e.logger.Warn("GetHTTPBalance error", "error", err.Error())
		return
	}
	e.logger.Debug("HTTP Res", "res", string(res))

	var parser fastjson.Parser

	data, err := parser.ParseBytes(res)
	if err != nil {
		e.logger.Warn("parse ws message error", "error", err.Error())
		return
	}

	balances := data.GetArray("assets")
	for i := range balances {
		asset := string(balances[i].GetStringBytes("asset"))
		free := utils.StringToFloat64(string(balances[i].GetStringBytes("maxWithdrawAmount")))
		locked := utils.StringToFloat64(string(balances[i].GetStringBytes("locked")))
		balance.Update(asset, &exchange.Balance{Free: free, Locked: locked})
	}

	return
}

func (e *BinancePerpsExchange) GetPosition() (position *exchange.AllPosition, err error) {
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
func (e *BinancePerpsExchange) SetLeverage(symbol exchange.SymbolName, lev int) (err error) {
	err = errors.New("spot exchange have not SetLeverage()")
	return
}
func (e *BinancePerpsExchange) SetAllLeverage(lev int) (err error) {
	err = errors.New("spot exchange have not SetAllLeverage()")
	return
}

/* ================================================ websocket订阅状态更新数据 ================================================ */

func (e *BinancePerpsExchange) GetRateLimitC() chan *exchange.RateLimit {
	return e.rateLimitC
}

/* ================================================ websocket订阅业务驱动数据 ================================================ */

func (e *BinancePerpsExchange) Test() {
	url := e.GetBaseUrl() + "/sapi/v1/sub-account/subAccountApi/ipRestriction"
	e.Send(url, http.MethodGet, map[string]string{
		"email":            "624_virtual@cqmvhds0managedsub.com",
		"subAccountApiKey": "L5uh9sCbrThUpiRrrYOSEhOp1lgzLG8G4S9SFDbuSPZfIGRumplPlrRG9ElrPLnM",
		"timestamp":        strconv.FormatInt(time.Now().UnixMilli(), 10),
	}, nil)
}

var _ exchange.Exchange = (*BinancePerpsExchange)(nil)
var _ exchange.Contractor = (*BinancePerpsExchange)(nil)
