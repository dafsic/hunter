package exchange

/* 交易规范 */

import (
	"sync"
)

type SymbolName string // 交易对字符串格式，业务层使用，方便策略跨所移植，格式为 "BTC-USDT"

func NewSymbolName(baseAsset, quoteAsset string) SymbolName {
	return SymbolName(baseAsset + "-" + quoteAsset)
}

/* =============================== 交易对规范 =============================== */
type SymbolInfo struct {
	SymbolName          SymbolName   // 打印名称
	NameInExchange      string       // 在交易所的原始名称
	BaseAsset           string       // 基础资产 (前缀，如BTC-USDT中的BTC)
	BaseAssetInExchange string       // 交易所里的基础资产（有些所可能会有数量前缀）
	QuoteAsset          string       // 计价资产 (后缀，如BTC-USDT中的USDT)
	InstType            ExchangeType // 产品类型:合约、现货
	PricePrecision      int32        // 价格的步进值，即小数点位数
	QuantityPrecision   int32        // 数量的步进值，即小数点位数
	CtVal               float64      // 合约面值
	CtMult              float64      // 合约乘数
	MinValue            float64      // 最小下单价值
}

/* =============================== 交易所全部SymbolInfo =============================== */
type AllSymbolsInfo struct {
	allSymbolsForSymbolName     map[SymbolName]*SymbolInfo // SymbolName 查 SymbolInfo
	allSymbolsForNameInExchange map[string]*SymbolInfo     // NameInExchange 查 SymbolInfo
	lock                        *sync.RWMutex
}

func NewAllSymbolsInfo() *AllSymbolsInfo {
	return &AllSymbolsInfo{
		allSymbolsForSymbolName:     make(map[SymbolName]*SymbolInfo),
		allSymbolsForNameInExchange: make(map[string]*SymbolInfo),
		lock:                        new(sync.RWMutex),
	}
}

func (s *AllSymbolsInfo) Set(symbolInfo *SymbolInfo) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.allSymbolsForSymbolName[symbolInfo.SymbolName] = symbolInfo
	s.allSymbolsForNameInExchange[symbolInfo.NameInExchange] = symbolInfo
}

func (s *AllSymbolsInfo) Update(data *AllSymbolsInfo) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.allSymbolsForSymbolName = data.allSymbolsForSymbolName
	s.allSymbolsForNameInExchange = data.allSymbolsForNameInExchange
}

// nameInExchange 和 symbolName 两个参数任选其一，如果两个参数都传入，优先使用 nameInExchang
func (s *AllSymbolsInfo) Get(nameInExchang string, symbolName SymbolName) *SymbolInfo {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if symbolInfo, ok := s.allSymbolsForNameInExchange[nameInExchang]; ok {
		return &SymbolInfo{
			SymbolName:          symbolInfo.SymbolName,
			NameInExchange:      symbolInfo.NameInExchange,
			BaseAsset:           symbolInfo.BaseAsset,
			BaseAssetInExchange: symbolInfo.BaseAssetInExchange,
			QuoteAsset:          symbolInfo.QuoteAsset,
			InstType:            symbolInfo.InstType,
			PricePrecision:      symbolInfo.PricePrecision,
			QuantityPrecision:   symbolInfo.QuantityPrecision,
			CtVal:               symbolInfo.CtVal,
			CtMult:              symbolInfo.CtMult,
			MinValue:            symbolInfo.MinValue,
		}
	}

	if symbolInfo, ok := s.allSymbolsForSymbolName[symbolName]; ok {
		return &SymbolInfo{
			SymbolName:          symbolInfo.SymbolName,
			NameInExchange:      symbolInfo.NameInExchange,
			BaseAsset:           symbolInfo.BaseAsset,
			BaseAssetInExchange: symbolInfo.BaseAssetInExchange,
			QuoteAsset:          symbolInfo.QuoteAsset,
			InstType:            symbolInfo.InstType,
			PricePrecision:      symbolInfo.PricePrecision,
			QuantityPrecision:   symbolInfo.QuantityPrecision,
			CtVal:               symbolInfo.CtVal,
			CtMult:              symbolInfo.CtMult,
			MinValue:            symbolInfo.MinValue,
		}
	}

	return nil
}

func (s *AllSymbolsInfo) GetAllSymbol() ([]*SymbolInfo, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var symbols []*SymbolInfo
	for _, symbol := range s.allSymbolsForSymbolName {
		symbols = append(symbols, &SymbolInfo{
			SymbolName:          symbol.SymbolName,
			NameInExchange:      symbol.NameInExchange,
			BaseAsset:           symbol.BaseAsset,
			BaseAssetInExchange: symbol.BaseAssetInExchange,
			QuoteAsset:          symbol.QuoteAsset,
			InstType:            symbol.InstType,
			PricePrecision:      symbol.PricePrecision,
			QuantityPrecision:   symbol.QuantityPrecision,
			CtVal:               symbol.CtVal,
			CtMult:              symbol.CtMult,
			MinValue:            symbol.MinValue,
		})
	}
	return symbols, true
}

func (s *AllSymbolsInfo) GetAllSymbolName() ([]SymbolName, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var symbolNames []SymbolName
	for symbol := range s.allSymbolsForSymbolName {
		symbolNames = append(symbolNames, symbol)
	}
	return symbolNames, true
}

// 根据Base 获取USDT区的交易规范
func (s *AllSymbolsInfo) GetUsdtSymbolForBase(base string) *SymbolInfo {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.Get("", SymbolName(base+"-USDT"))
}

// 通过交易所名称获取打印名称
func (s *AllSymbolsInfo) GetSymbolName(nameInExchange string) (SymbolName, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if symbolInfo, ok := s.allSymbolsForNameInExchange[nameInExchange]; ok {
		return symbolInfo.SymbolName, true
	}
	return "", false
}

// 通过打印名称获取交易所名称
func (s *AllSymbolsInfo) GetNameInExchange(symbolName SymbolName) (string, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if symbolInfo, ok := s.allSymbolsForSymbolName[symbolName]; ok {
		return symbolInfo.NameInExchange, true
	}
	return "", false
}
