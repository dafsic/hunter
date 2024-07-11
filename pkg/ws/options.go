package ws

import "time"

type wsCfg struct {
	Proxy     string
	WriteWait time.Duration
}

type Option func(*wsCfg)

func WithProxy(proxy string) Option {
	return func(wc *wsCfg) {
		wc.Proxy = proxy
	}
}

func WithWriteWait(t time.Duration) Option {
	return func(wc *wsCfg) {
		wc.WriteWait = t
	}
}
