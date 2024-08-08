package exchange

import (
	"time"
)

/* ===================================================== 最优挂单 ===================================================== */

type BookTicker struct {
	symbolName SymbolName // 交易对
	askPrice   float64    // 买一价格
	askQty     float64    // 买一数量
	bidPrice   float64    // 卖一价格
	bidQty     float64    // 卖一数量
	updateID   int64      // 更新ID
	uTime      int64      // 数据在交易所的更新时间
	resTime    int64      // 数据接收时间

	// rlock *sync.RWMutex
}

func NewBookTicker(
	symbolName SymbolName,
	askPrice float64,
	askQty float64,
	bidPrice float64,
	bidQty float64,
	updateID int64,
	uTime int64,
) *BookTicker {
	return &BookTicker{
		symbolName: symbolName,
		askPrice:   askPrice,
		askQty:     askQty,
		bidPrice:   bidPrice,
		bidQty:     bidQty,
		updateID:   updateID,
		uTime:      uTime,
		resTime:    time.Now().UnixMilli(),
		// rlock:      new(sync.RWMutex),
	}
}

func (b *BookTicker) Get() (symbolName SymbolName, askPrice float64, askQty float64, bidPrice float64, bidQty float64, updateID int64, uTime int64) {
	return b.symbolName, b.askPrice, b.askQty, b.bidPrice, b.bidQty, b.updateID, b.uTime
}

func (b *BookTicker) Update(
	askPrice float64,
	askQty float64,
	bidPrice float64,
	bidQty float64,
	updateID int64,
	uTime int64,
) bool {
	if b.updateID < updateID {
		b.askPrice = askPrice
		b.askQty = askQty
		b.bidPrice = bidPrice
		b.bidQty = bidQty
		b.updateID = updateID
		b.uTime = uTime
		b.resTime = time.Now().UnixMilli()

		return true
	}

	return false
}
