package model

/* 枚举 */

type MyLogLevel string      // 日志级别
type ExchangeName string    // 交易所名称
type ExchangeType string    // 交易对类型
type OrderType string       // 订单类型
type OrderSide string       // 订单方向
type OrderState string      // 订单状态
type OrderForce string      // 订单时效
type OrderUpdateType string // 订单更新类型
type RateLimitsL int        // 限速分级

const (
	Debug    MyLogLevel = "DEBUG"
	Info     MyLogLevel = "INFO"
	Warning  MyLogLevel = "WARNING"
	Error    MyLogLevel = "ERROR"
	Critical MyLogLevel = "CRITICAL"
)
const (
	Binance      ExchangeName = "Binance"
	BinanceSpot  ExchangeName = "BinanceSpot"
	BinancePerps ExchangeName = "BinancePerps"

	Okx      ExchangeName = "Okx"
	OkxSpot  ExchangeName = "OkxSpot"
	OkxPerps ExchangeName = "OkxPerps"

	Huobi      ExchangeName = "Huobi"
	HuobiSpot  ExchangeName = "HuobiSpot"
	HuobiPerps ExchangeName = "HuobiPerps"

	Bybit      ExchangeName = "Bybit"
	BybitSpot  ExchangeName = "BybitSpot"
	BybitPerps ExchangeName = "BybitPerps"

	Bitget      ExchangeName = "Bitget"
	BitgetSpot  ExchangeName = "BitgetSpot"
	BitgetPerps ExchangeName = "BitgetPerps"

	Gate      ExchangeName = "Gate"
	GateSpot  ExchangeName = "GateSpot"
	GatePerps ExchangeName = "GatePerps"
)
const (
	Spot  ExchangeType = "SPOT"  // 现货
	Perps ExchangeType = "PERPS" // 合约
)
const (
	Market       OrderType = "MARKET"       // 市价单
	Limit        OrderType = "LIMIT"        // 限价单
	Limit_Market OrderType = "LIMIT_MARKET" // 只做Market订单
)
const (
	FOK OrderForce = "FOK" // 无法立即全部成交就撤单
	GTC OrderForce = "GTC" // 成交为止。订单会一直有效，直到被成交或者取消
	IOC OrderForce = "IOC" // 无法立即成交的部分就撤销。订单在失效前会尽量多的成交。
)
const (
	Buy  OrderSide = "BUY"
	Sell OrderSide = "SELL"
)
const (
	New              OrderState = "NEW"              // 新建订单
	Fail             OrderState = "FAIL"             // 创建订单失败
	Partially_Filled OrderState = "PARTIALLY_FILLED" // 部分成交
	Filled           OrderState = "FILLED"           // 完全成交
	Canceled         OrderState = "CANCELED"         // 由用户撤销
	Expired          OrderState = "EXPIRED"          // 由系统撤销
	ExchangeError    OrderState = "EXCHANGEERROR"    // 交易所异常
)
const (
	Update_New      OrderUpdateType = "UPDATE_NEW"
	Update_Canceled OrderUpdateType = "UPDATE_CANCELED"
	Update_Rejected OrderUpdateType = "UPDATE_REJECTED"
	Update_TRADE    OrderUpdateType = "UPDATE_TRADE"
	Update_Expired  OrderUpdateType = "UPDATE_EXPIRED"
)
const (
	RateLimitsLMinute RateLimitsL = iota
	RateLimitsLSecond
	RateLimitsLSecond10
	RateLimitsLHour
	RateLimitsLDay
)
