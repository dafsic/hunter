package spot

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/pkg/ws"
	"github.com/dafsic/hunter/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fastjson"
)

type SpotOrderController struct {
	e           exchange.Exchange
	client      ws.Client
	stopC       chan struct{}
	responseMap sync.Map
	rLock       sync.RWMutex
}

func NewSpotOrderController(e exchange.Exchange, url, localIP, apiKey string, secretKey ed25519.PrivateKey, heartBeat int) (*SpotOrderController, error) {
	oc := &SpotOrderController{
		e:     e,
		stopC: make(chan struct{}),
	}

	client, err := e.WebSocketManager().NewClient(url, localIP, oc.callback, heartBeat, false)
	if err != nil {
		return nil, err
	}
	oc.client = client
	err = oc.login(apiKey, secretKey)
	if err != nil {
		return nil, err
	}

	// 处理断线重连
	go func() {
		var (
			disconnectC <-chan struct{}
			err         error
		)
		disconnectC = oc.client.IsDisconnect()
		for {
			select {
			case <-oc.stopC:
				return
			case <-disconnectC:
				oc.rLock.Lock()
				for {
					client, _ = e.WebSocketManager().NewClient(url, localIP, oc.callback, heartBeat, false)
					err = oc.login(apiKey, secretKey)
					if err != nil {
						e.Logger().Warn("websocket reconnected fail", "url", url, "error", err)
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}
				disconnectC = client.IsDisconnect()
				oc.rLock.Unlock()
			}
		}
	}()

	return oc, nil
}

func (oc *SpotOrderController) Close() {
	oc.rLock.Lock()
	defer oc.rLock.Unlock()

	close(oc.stopC)
	oc.client.Close()
}

// PlaceOrder 下单
func (oc *SpotOrderController) PlaceOrder(newOrder exchange.NewOrder) (*exchange.OrderDetail, error) {
	var (
		id    = strconv.FormatInt(time.Now().UnixNano(), 10)
		respC = make(chan *fastjson.Value)
	)
	oc.responseMap.Store(id, respC)

	// 构建下单参数
	symbolInfo := oc.e.GetSymbolInfo(newOrder.Symbol)
	if symbolInfo == nil {
		return nil, errors.New("symbol:" + string(newOrder.Symbol) + " not found")
	}

	side := newOrder.Side.String()
	timeInForce := newOrder.TimeInForce.String()

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
	case exchange.Limit:
		params["type"] = "LIMIT"
		params["price"] = utils.Float64ToString(utils.FormatFloat(newOrder.Price, symbolInfo.PricePrecision))
		params["timeInForce"] = timeInForce
	case exchange.Market:
		params["type"] = "MARKET"
	}

	sendMsg := map[string]any{
		"id":     id,
		"method": "order.place",
		"params": params,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return nil, fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	// 下单
	err = oc.sendMessage(payload)
	if err != nil {
		return nil, fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	// 接收返回
	data := <-respC

	orderDetail := &exchange.OrderDetail{
		ClientOrderID: newOrder.ClientOrderID,
		Symbol:        newOrder.Symbol,
		Side:          newOrder.Side,
		OType:         newOrder.OType,
		OrigQty:       newOrder.Quantity,
		OrigPrice:     newOrder.Price,
	}
	orderDetail.State.Set(string(data.GetStringBytes("result", "status")))
	if orderDetail.State == exchange.Fail {
		orderDetail.ErrCode = data.GetInt("error", "code")
		orderDetail.ErrMsg = string(data.GetStringBytes("error", "msg"))
		return orderDetail, nil
	}

	executedQty := utils.StringToFloat64(string(data.GetStringBytes("result", "executedQty")))
	executedMoney := utils.StringToFloat64(string(data.GetStringBytes("result", "cummulativeQuoteQty")))
	orderDetail.ExecutedQty = executedQty
	orderDetail.ExecutedMoney = executedMoney
	if executedQty != 0 {
		orderDetail.ExecutedAvPrice = executedMoney / executedQty
	}

	return orderDetail, nil
}

func (oc *SpotOrderController) CancelOrder(clientOrderID string) error {
	var (
		resC = make(chan *fastjson.Value)
		id   = utils.GetUUID()
	)
	oc.responseMap.Store(id, resC)

	sendMsg := map[string]any{
		"id": id,
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return err
	}

	// 下单
	err = oc.sendMessage(payload)
	if err != nil {
		return err
	}

	msg := <-resC
	_ = msg

	return nil
}

func (oc *SpotOrderController) CancelAllOrder(symbol exchange.SymbolName) error {
	var (
		resC = make(chan *fastjson.Value)
		id   = utils.GetUUID()
	)
	oc.responseMap.Store(id, resC)

	sendMsg := map[string]any{
		"id":      id,
		"method":  "order.cancelAll",
		"symbols": []string{string(symbol)},
	}

	payload, err := jsoniter.Marshal(sendMsg)
	if err != nil {
		return err
	}

	// 下单
	err = oc.sendMessage(payload)
	if err != nil {
		return err
	}

	msg := <-resC
	_ = msg

	return nil
}

func (oc *SpotOrderController) ReplaceOrder(cancelOrigClientOrderId string, newOrder exchange.NewOrder) (*exchange.OrderDetail, error) {
	return nil, nil
}

func (oc *SpotOrderController) callback(msg []byte) {
	var parser fastjson.Parser

	data, err := parser.ParseBytes(msg)
	if err != nil {
		oc.e.Logger().Warn("parse ws message error", "error", err.Error())
		return
	}

	id := string(data.GetStringBytes("id"))

	if c, ok := oc.responseMap.Load(id); ok {
		c.(chan *fastjson.Value) <- data
		oc.responseMap.Delete(id)
	}

	oc.e.Logger().Debug("websocket api event", "res", string(msg))
}

func (oc *SpotOrderController) login(apiKey string, secretKey ed25519.PrivateKey) error {
	params := map[string]string{
		"apiKey":    apiKey,
		"timestamp": strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
	}
	params["signature"] = utils.GetED25519Base64SignStr([]byte((utils.GetAndEqJionString(params))), secretKey)
	data := map[string]any{
		"id":     "login",
		"method": "session.logon",
		"params": params,
	}

	payload, err := jsoniter.Marshal(data)
	if err != nil {
		return err
	}

	err = oc.sendMessage(payload)
	if err != nil {
		return err
	}

	return nil
}

func (oc *SpotOrderController) sendMessage(msg []byte) error {
	oc.rLock.RLock()
	defer oc.rLock.RUnlock()

	return oc.client.SendMessage(msg)
}

var _ exchange.OrderController = (*SpotOrderController)(nil)
