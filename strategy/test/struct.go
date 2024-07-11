package test

/* 策略资源结构体 */

import (
	"sync"
)

type updateID struct {
	m int
	l *sync.Mutex
}

func newUpdateID() *updateID {
	return &updateID{
		m: 0,
		l: new(sync.Mutex),
	}
}

func (u *updateID) Keep(id int) bool {
	u.l.Lock()
	defer u.l.Unlock()

	if id > u.m {
		u.m = id
		return true
	}

	return false
}
