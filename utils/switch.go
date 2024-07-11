package utils

import "sync/atomic"

type Switcher int32

func (s *Switcher) On() {
	atomic.CompareAndSwapInt32((*int32)(s), 0, 1)
}

func (s *Switcher) Off() {
	atomic.CompareAndSwapInt32((*int32)(s), 1, 0)
}

func (s *Switcher) State() Switcher {
	return Switcher(atomic.LoadInt32((*int32)(s)))
}
