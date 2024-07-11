package exchange

import (
	"github.com/dafsic/hunter/exchange/binance"
	"github.com/dafsic/hunter/exchange/model"
	"github.com/valyala/fastjson"
)

// 交易所接口
type Exchange interface {

	/* ================================================ 获取实时数据 ================================================ */
	// 该类方法会发起一个请求，向交易所拿取最新的数据
	// 分为公共行情数据和私有账户数据
	// 账户数据建议使用下面的 websocket订阅状态类数据，实时性更强

	GetHTTPServerTime() (timeStamp int64, err error)                                     // 获取服务器当前毫秒级时间戳
	GetHTTPPrice(symbol model.SymbolName) (price float64, err error)                     // 获取最新价格
	GetHTTPBookTicker(symbol model.SymbolName) (bookTicker *model.BookTicker, err error) // 获取最优挂单
	GetHTTPBalance() (balance *model.AllBalance, err error)
	GetHTTPPosition() (position *model.AllPosition, err error)

	/* ================================================ 修改账户设置 ================================================ */
	// 该类方法会发起一个HTTP请求，向交易所提交修改账户设置

	SetPositionSide(isDualMode bool) (err error)              // 设置合约持仓模式 (合约专用 isDualMode=true 开启双向持仓)
	SetMarginType(isCross bool) (err error)                   // 设置合约保证金模式 (合约专用 isCross=true 开启全仓)
	SetLeverage(symbol model.SymbolName, lev int) (err error) // 设置合约杠杆 (合约专用)
	SetAllLeverage(lev int) (err error)                       // 设置合约所有交易对的杠杆 (合约专用)

	/* ================================================ websocket订阅状态更新数据 ================================================ */
	// 返回一个 结构体实例的指针，具体类型视方法而定
	// 底层 由交易所websocket连接自动更新结构体的数据
	// 上层 由业务层直接调用结构体实例的相应方法即可获取到当前的最新数据
	// 特点: 接口层需要处理每一次数据推送，处理方式通常为更新结构体实例中的相应数据，业务层只在需要的时候来获取数据的当前状态
	// 如: 当前资产余额、当前合约持仓、当前挂单、交易规范等
	// 注意事项：高频场景下，由于数据更新依赖于websocket的推送，会有一定延迟

	GetRateLimitC() chan error                                       // 获取Ip or order 超限信号监听通道
	GetStructOfAllBanlance() (allBalance *model.AllBalance)          // 获取账户余额结构体 (自动更新)
	GetStructOfAllPosition() (allPosition *model.AllPosition)        // 获取账户持仓结构体 (自动更新)
	GetStructOfAllSymbolInfo() (allSymbolInfo *model.AllSymbolsInfo) // 获取交易对规范结构体 (自动更新)
	SetSubBalanceCallback(callback func(data *fastjson.Value))       // 设置ws账户资产更新推送的回调函数
	SetSubOrderCallback(callback func(data *fastjson.Value))         // 设置ws订单更新推送的回调函数

	/* ================================================ websocket订阅业务驱动数据 ================================================ */
	// 返回chan，传输的数据类型视具体方法而定
	// 底层 由交易所websokcet连接来生产数据
	// 上层 由业务层来消费数据
	// 特点: 业务层需要处理每一次数据推送，通常是驱动业务层逻辑前进的数据

	SubBookTicker(symbols []model.SymbolName, callback func(msg []byte), isGoroutine bool) error // 订阅最优挂单
	// 订阅最优挂单 (高速行情)。
	// numConnPerNIC: 每个网卡的连接数；
	// symbols: 交易对列表；
	// callback: 数据推送回调函数；
	// isGoroutine: 是否在新的goroutine中执行。
	// SubBookTickerFast(numConnPerNIC int, symbols []model.SymbolName, callback func(msg []byte), isGoroutine bool) (err error)

	/* ================================================ 订单操作 ================================================ */
	// 根据交易所的支持类型自动选择HTTP还是wsbsocket

	NewOrder(newOrder model.NewOrder) (orderDetail *model.OrderDetail, err error)                           // 新建订单
	GetOrder(symbol model.SymbolName, myOrderID string) (orderDetail *model.OrderDetail, err error)         // 查询订单
	DelOrder(symbol model.SymbolName, myOrderID string) (err error)                                         // 撤销挂单
	DelAllOrder(symbol model.SymbolName) (err error)                                                        // 撤销交易对全部挂单
	DelReplace(newOrder model.NewOrder, cancelMyOrderID string) (orderDetail *model.OrderDetail, err error) // 撤单再下单

	/* ================================================ 测试 ================================================ */
	Test()
}

var _ Exchange = (*binance.BinanceSpotExchange)(nil)
