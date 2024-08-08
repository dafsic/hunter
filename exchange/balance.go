package exchange

/* 资产余额 */

import (
	"sync"
)

type Balance struct {
	Asset  string
	Free   float64
	Locked float64
}

type AllBalance struct {
	balanceMap map[string]*Balance // 币种名称：详情
	rLock      *sync.RWMutex
}

func NewAllBalance() *AllBalance {
	return &AllBalance{
		balanceMap: make(map[string]*Balance, 0),
		rLock:      new(sync.RWMutex),
	}
}

func (b AllBalance) Update(coin string, balance *Balance) {
	b.rLock.Lock()
	defer b.rLock.Unlock()

	b.balanceMap[coin] = balance
}
func (b AllBalance) GetLock(coin string) (lock float64) {
	b.rLock.RLock()
	defer b.rLock.RUnlock()

	if _, ok := b.balanceMap[coin]; ok {
		lock = b.balanceMap[coin].Locked
	}

	return
}
func (b AllBalance) GetFree(coin string) (free float64) {
	b.rLock.RLock()
	defer b.rLock.RUnlock()

	if _, ok := b.balanceMap[coin]; ok {
		free = b.balanceMap[coin].Free
	}

	return
}
func (b AllBalance) GetSumBalance(coin string) (balance float64) {
	b.rLock.RLock()
	defer b.rLock.RUnlock()

	if _, ok := b.balanceMap[coin]; ok {
		balance = b.balanceMap[coin].Free + b.balanceMap[coin].Locked
	}

	return
}
