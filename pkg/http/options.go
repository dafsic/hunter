package http

type httpCfg struct {
	Proxy string
}

type Option func(*httpCfg)

func WithProxy(proxy string) Option {
	return func(wc *httpCfg) {
		wc.Proxy = proxy
	}
}
