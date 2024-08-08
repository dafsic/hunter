package exchange

/* 合约持仓 */

import (
	"sync"
)

type Position struct {
	Symbol SymbolName // 交易对
	Side   OrderSide  // 持仓方向
	Amount float64    // 持仓数量
	Cost   float64    // 持仓成本
}

type AllPosition struct {
	positions map[SymbolName]*Position
	rLock     *sync.RWMutex
}

func NewAllPosition() *AllPosition {
	return &AllPosition{
		positions: make(map[SymbolName]*Position, 0),
		rLock:     new(sync.RWMutex),
	}
}

func (p AllPosition) Update(pos *Position) {
	p.rLock.Lock()
	defer p.rLock.Unlock()

	p.positions[pos.Symbol] = pos

}
func (p AllPosition) GetPosition(symbol SymbolName) (pos *Position) {
	pos = &Position{}
	p.rLock.RLock()
	defer p.rLock.RUnlock()

	if pos, ok := p.positions[symbol]; ok {
		pos.Symbol = p.positions[symbol].Symbol
		pos.Amount = p.positions[symbol].Amount
		pos.Cost = p.positions[symbol].Cost
		pos.Side = p.positions[symbol].Side
	}
	return
}
func (p AllPosition) GetPositions() (poss []*Position) {
	p.rLock.RLock()
	defer p.rLock.RUnlock()

	poss = make([]*Position, 0, len(p.positions))
	for _, v := range p.positions {

		var pos = &Position{}
		pos.Amount = v.Amount
		pos.Cost = v.Cost
		pos.Symbol = v.Symbol
		pos.Side = v.Side

		poss = append(poss, pos)
	}
	return
}
