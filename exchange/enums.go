package exchange

// =================交易所名称=================
type ExchangeName string

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

func (n ExchangeName) String() string {
	return string(n)
}

// =================交易类型=================
type ExchangeType uint8

const (
	_     ExchangeType = iota
	Spot               // 现货
	Perps              // 合约
)

func (t ExchangeType) String() string {
	switch t {
	case Spot:
		return "SPOT"
	case Perps:
		return "PERPS"
	default:
		return ""
	}
}

// =================订单方向=================
type OrderSide uint8

const (
	_ OrderSide = iota
	Buy
	Sell
)

func (s OrderSide) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return ""
	}
}

func (s *OrderSide) Set(str string) {
	switch str {
	case "BUY":
		*s = Buy
	case "SELL":
		*s = Sell
	default:
		panic("OrderSide Set Error")
	}
}

// =================订单类型=================
type OrderType uint8

const (
	_           OrderType = iota
	Market                // 市价单
	Limit                 // 限价单
	LimitMarket           // 只做Marker订单
)

func (t OrderType) String() string {
	switch t {
	case Market:
		return "MARKET"
	case Limit:
		return "LIMIT"
	case LimitMarket:
		return "LIMIT_MARKET"
	default:
		return ""
	}
}

// =================订单状态=================
type OrderState uint8

const (
	_               OrderState = iota
	New                        // 新建订单
	Fail                       // 创建订单失败
	PartiallyFilled            // 部分成交
	Filled                     // 完全成交
	Canceled                   // 由用户撤销
	Expired                    // 由系统撤销
	ExchangeError              // 交易所异常
)

func (s OrderState) String() string {
	switch s {
	case New:
		return "NEW"
	case Fail:
		return "FAIL"
	case PartiallyFilled:
		return "PARTIALLY_FILLED"
	case Filled:
		return "FILLED"
	case Canceled:
		return "CANCELED"
	case Expired:
		return "EXPIRED"
	case ExchangeError:
		return "EXCHANGEERROR"
	default:
		return ""
	}
}

func (s *OrderState) Set(str string) {
	switch str {
	case "NEW":
		*s = New
	case "PARTIALLY_FILLED":
		*s = PartiallyFilled
	case "FILLED":
		*s = Filled
	case "CANCELED":
		*s = Canceled
	case "EXPIRED":
		*s = Expired
	case "EXCHANGEERROR":
		*s = ExchangeError
	default:
		panic("OrderState Set Error")
	}
}

// =================订单时效=================
type OrderForce uint8

const (
	_   OrderForce = iota
	FOK            // 无法立即全部成交就撤单
	GTC            // 成交为止。订单会一直有效，直到被成交或者取消
	IOC            // 无法立即成交的部分就撤销。订单在失效前会尽量多的成交。
)

func (f OrderForce) String() string {
	switch f {
	case FOK:
		return "FOK"
	case GTC:
		return "GTC"
	case IOC:
		return "IOC"
	default:
		return ""
	}
}

// =================订单更新类型=================
type OrderUpdateType uint8

const (
	_ OrderUpdateType = iota
	UpdateNew
	UpdateCanceled
	UpdateRejected
	UpdateTRADE
	UpdateExpired
)

func (t OrderUpdateType) String() string {
	switch t {
	case UpdateNew:
		return "UPDATE_NEW"
	case UpdateCanceled:
		return "UPDATE_CANCELED"
	case UpdateRejected:
		return "UPDATE_REJECTED"
	case UpdateTRADE:
		return "UPDATE_TRADE"
	case UpdateExpired:
		return "UPDATE_EXPIRED"
	default:
		return ""
	}
}

func (t *OrderUpdateType) Set(str string) {
	switch str {
	case "UPDATE_NEW":
		*t = UpdateNew
	case "UPDATE_CANCELED":
		*t = UpdateCanceled
	case "UPDATE_REJECTED":
		*t = UpdateRejected
	case "UPDATE_TRADE":
		*t = UpdateTRADE
	case "UPDATE_EXPIRED":
		*t = UpdateExpired
	default:
		panic("OrderUpdateType Set Error")
	}
}

type RateLimitsL int // 限速分级
const (
	RateLimitsLMinute RateLimitsL = iota
	RateLimitsLSecond
	RateLimitsLSecond10
	RateLimitsLHour
	RateLimitsLDay
)
