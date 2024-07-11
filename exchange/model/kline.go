package model

/* ===================================================== K线 ===================================================== */

type Kline struct {
	OpenTime int64   // 开盘时间
	Open     float64 // 开盘价
	High     float64 // 最高价
	Low      float64 // 最低价
	Close    float64 // 收盘价
	VolBase  float64 // 成交量
	VolQuote float64 // 交易额
	IsClose  bool    // 是否收盘
	ResTime  int64   // 数据接收时间
}

/* ===================================================== 池化 ===================================================== */
