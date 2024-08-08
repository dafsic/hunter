package exchange

import (
	"sync"
)

/* 订单 */

// 新建订单
type NewOrder struct {
	OType         OrderType  // 订单类型
	Side          OrderSide  // 订单方向
	TimeInForce   OrderForce // 有效时间
	ReduceOnly    bool       // 是否只减仓
	Symbol        SymbolName // 交易对
	Quantity      float64    // 统一为币种名义数量，如果合约下单单位为张数，需要使用合约乘数进行转换
	QuoteOrderQty float64    // 市价买单的总金额
	Price         float64    // 价格
	ClientOrderID string     // 自定义订单ID
}

// 订单详情
type OrderDetail struct {
	Symbol    SymbolName // 交易对
	State     OrderState // 订单状态
	Side      OrderSide  // 交易方向
	OType     OrderType  // 订单类型
	OrigQty   float64    // 原始订单数量
	OrigPrice float64    // 原始订单价格

	ExecutedQty     float64 // 累计已成交数量
	ExecutedMoney   float64 // 累计已成交金额
	ExecutedAvPrice float64 // 已成交部分的成交均价

	NowOrderTime int64 // 订单创建时间
	UpdateTime   int64 // 订单最后更新时间

	ClientOrderID string // 自定义订单ID
	ErrCode       int    // 错误码
	ErrMsg        string // 错误信息
}

// 订单详情map key:订单ID value:订单详情
type OrderDetailMap struct {
	m           map[int]map[string]*OrderDetail // 两个订单map，每个map长度均为1000
	updateIndex int                             // 当前更新的map索引
	l           *sync.RWMutex
}

func NewOrderDetailMap() *OrderDetailMap {

	m1 := make(map[string]*OrderDetail, 1000)
	m2 := make(map[string]*OrderDetail, 1000)

	m := make(map[int]map[string]*OrderDetail, 2)
	m[1] = m1
	m[2] = m2

	return &OrderDetailMap{
		m:           m,
		updateIndex: 1,
		l:           new(sync.RWMutex),
	}
}
func (o *OrderDetailMap) Add(k string, v *OrderDetail) {
	o.l.Lock()
	defer o.l.Unlock()

	// 如果当前的存储池满了, 将旧的存储池清空, 并切换到另一个存储池
	if len(o.m[o.updateIndex]) >= 1000 {
		o.updateIndex = 3 - o.updateIndex
		o.m[o.updateIndex] = make(map[string]*OrderDetail, 1000)
	}

	o.m[o.updateIndex][k] = v
}

// 通过自定义ID来查询订单
func (o *OrderDetailMap) Get(k string) (*OrderDetail, bool) {
	o.l.RLock()
	defer o.l.RUnlock()

	if v, ok := o.m[o.updateIndex][k]; ok {
		return &OrderDetail{
			Symbol:          v.Symbol,
			State:           v.State,
			Side:            v.Side,
			OType:           v.OType,
			OrigQty:         v.OrigQty,
			OrigPrice:       v.OrigPrice,
			ExecutedQty:     v.ExecutedQty,
			ExecutedMoney:   v.ExecutedMoney,
			ExecutedAvPrice: v.ExecutedAvPrice,
			NowOrderTime:    v.NowOrderTime,
			UpdateTime:      v.UpdateTime,
			ClientOrderID:   v.ClientOrderID,
			ErrCode:         v.ErrCode,
			ErrMsg:          v.ErrMsg,
		}, true
	}

	if v, ok := o.m[3-o.updateIndex][k]; ok {
		return &OrderDetail{
			Symbol:          v.Symbol,
			State:           v.State,
			Side:            v.Side,
			OType:           v.OType,
			OrigQty:         v.OrigQty,
			OrigPrice:       v.OrigPrice,
			ExecutedQty:     v.ExecutedQty,
			ExecutedMoney:   v.ExecutedMoney,
			ExecutedAvPrice: v.ExecutedAvPrice,
			NowOrderTime:    v.NowOrderTime,
			UpdateTime:      v.UpdateTime,
			ClientOrderID:   v.ClientOrderID,
			ErrCode:         v.ErrCode,
			ErrMsg:          v.ErrMsg,
		}, true
	}

	return nil, false
}

// 订单更新推送
// ! 不推荐使用，因为每次推送都会解析全部字段，建议直接使用原始数据，需要什么字段自己解析，减少解析次数，提高性能
type OrderUpdate struct {
	ClientID   string          // 自定义订单ID
	Symbol     SymbolName      // 交易对
	UpdateType OrderUpdateType // 更新类型
	TradeQty   float64         // 本次成交数量
	TradePrice float64         // 本次成交价格
	TradeTime  int64           // 本次成交时间
}

// type OrderUpdate struct {
// 	UpdateType OrderUpdateType // 更新类型
// 	OrderState OrderState      // 订单状态
// 	OrderSide  OrderSide       // 订单方向
// 	OrderType  OrderType       // 订单类型
// 	Symbol     SymbolName      // 交易对
// 	ClientID   string          // 自定义订单ID
// 	OrderID    int64           // 订单ID
// 	TradeID    int64           // 成交ID
// 	OrigPrice  float64         // 原始订单价格
// 	OrigQty    float64         // 原始订单数量
// 	TradeQty   float64         // 本次成交数量
// 	TradePrice float64         // 本次成交价格
// 	FeeQty     float64         // 手续费数量
// 	FeeAsset   string          // 手续费资产
// 	MakeTime   int64           // 订单创建时间
// 	TradeTime  int64           // 本次成交时间
// 	IsMaker    bool            // 是否为挂单成交
// 	SumQty     float64         // 累计已成交数量
// 	SumMoney   float64         // 累计已成交金额
// }
