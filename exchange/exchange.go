package exchange

import (
	"crypto/ed25519"

	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"
)

// 订单控制器接口，根据交易所的支持类型自动选择HTTP还是websocket
type OrderController interface {
	Close()
	// 下单
	PlaceOrder(newOrder NewOrder) (*OrderDetail, error)
	// 撤单
	CancelOrder(clientOrderID string) (err error)
	// 撤销交易对全部挂单
	CancelAllOrder(symbol SymbolName) (err error)
	// 撤单再下单
	ReplaceOrder(cancelOrigClientOrderId string, newOrder NewOrder) (*OrderDetail, error)
	// 查询订单
	// QueryOrder(clientOrderID string) (order *model.OrderDetail, err error)
}

// 行情控制器接口
type MarketController interface {
	Close()
	BookTickerChannel() <-chan *BookTicker
}

// 账户控制器接口
type AccountController interface {
	Close()
	// // 订阅用户持仓变化
	// SubBalance(callback func(msg []byte)) (err error)
	// // 订阅用户订单状态变化
	// SubOrder(callback func(msg []byte)) (err error)
	BalanceChannel() <-chan *Balance
	OrderUpdateChannel() <-chan *OrderUpdate
}

// 合约交易所专有接口
type Contractor interface {
	// 获取账户持仓
	GetPosition() (position *AllPosition, err error)
	// 设置合约持仓模式
	SetPositionSide(isDualMode bool) (err error)
	// 设置合约保证金模式
	SetMarginType(isCross bool) (err error)
	// 设置合约杠杆
	SetLeverage(symbol SymbolName, lev int) (err error)
	// 设置合约所有交易对的杠杆
	SetAllLeverage(lev int) (err error)
}

// 交易所接口
type Exchange interface {
	// 获取交易所日志记录器
	Logger() log.Logger
	// 获取交易所websocket管理器
	WebSocketManager() ws.Manager
	// 获取服务器当前毫秒级时间戳
	GetServerTime() (timeStamp int64, err error)
	// 获取交易所的基础url
	GetBaseUrl() string
	// 获取交易对规范信息
	GetSymbolInfo(symbolName SymbolName) *SymbolInfo
	// 根据交易所中的名字，获取交易对规范信息
	GetSymbloInfoByExchangeName(name string) *SymbolInfo
	// 获取账户余额
	GetBalance(apiKey, secretKey string) (balance *AllBalance, err error)
	// 创建行情控制器
	CreateMarketController(symbols []SymbolName, localIP string) (MarketController, error)
	// 创建订单控制器
	CreateOrderController(localIP string, apiKey string, secretKey ed25519.PrivateKey) (OrderController, error)
	// 创建账户控制器
	CreateAccountController(localIP, apiKey string) (AccountController, error)
	// 通过http向交易所发出请求
	Send(url string, method string, payload map[string]string, header map[string]string) (code int, res []byte, err error)
	SendWithApikey(url string, method string, payload map[string]string, apiKey string) (code int, res []byte, err error)
	SendWithSign(url string, method string, payload map[string]string, apiKey, secretKey string) (code int, res []byte, err error)
}
