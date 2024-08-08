package spot

import (
	"fmt"
	"strconv"

	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/utils"

	"github.com/valyala/fastjson"
)

func (e *BinanceSpotExchange) getAllSymbolInfo() (*exchange.AllSymbolsInfo, error) {
	var (
		parser  fastjson.Parser
		urlPath = e.getBaseUrl() + "/api/v3/exchangeInfo"
	)

	// urlPath += "?symbols=[\"" + strings.Join(symbols, `","`) + "\"]"

	_, res, err := e.send(urlPath, "GET", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	data, err := parser.ParseBytes(res)
	if err != nil {
		return nil, fmt.Errorf("%w%s", err, utils.LineInfo())
	}

	allSymbolsInfo := exchange.NewAllSymbolsInfo()

	for _, symbol := range data.GetArray("symbols") {
		if string(symbol.GetStringBytes("status")) != "TRADING" {
			continue
		}

		var (
			nameInExchange string
			baseAsset      string
			quoteAsset     string
			symbolName     exchange.SymbolName

			pricePrecision    int32
			quantityPrecision int32
			minValue          float64
		)

		for _, filter := range symbol.GetArray("filters") {
			switch string(filter.GetStringBytes("filterType")) {
			case "LOT_SIZE":
				quantityPrecision = utils.GetDecimalPlaces(string(filter.GetStringBytes("stepSize")))
			case "PRICE_FILTER":
				pricePrecision = utils.GetDecimalPlaces(string(filter.GetStringBytes("tickSize")))
			case "NOTIONAL":
				minValue, _ = strconv.ParseFloat(string(filter.GetStringBytes("minNotional")), 64)
			}
		}

		nameInExchange = string(symbol.GetStringBytes("symbol"))
		baseAsset = string(symbol.GetStringBytes("baseAsset"))
		quoteAsset = string(symbol.GetStringBytes("quoteAsset"))
		symbolName = exchange.NewSymbolName(baseAsset, quoteAsset)

		symbolInfo := &exchange.SymbolInfo{
			NameInExchange:      nameInExchange,
			BaseAsset:           baseAsset,
			BaseAssetInExchange: baseAsset,
			QuoteAsset:          quoteAsset,
			SymbolName:          symbolName,
			InstType:            exchange.Spot,
			PricePrecision:      pricePrecision,
			QuantityPrecision:   quantityPrecision,
			CtVal:               1,
			CtMult:              1,
			MinValue:            minValue,
		}

		allSymbolsInfo.Set(symbolInfo)
	}

	return allSymbolsInfo, nil
}
