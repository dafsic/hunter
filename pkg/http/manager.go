package http

import (
	"os"
	"strings"

	"github.com/dafsic/hunter/pkg/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type Manager interface {
	NewClient() *HTTPClient
}

type HTTPManager struct {
	logger log.Logger
	proxy  string
	client *fasthttp.Client
}

func New(l log.Logger, opts ...Option) *HTTPManager {
	cfg := httpCfg{
		Proxy: "",
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &HTTPManager{
		logger: l,
		proxy:  cfg.Proxy,
	}
}

func (m *HTTPManager) NewClient() *HTTPClient {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	return &HTTPClient{
		Req:    req,
		Resp:   resp,
		Client: m.client,
	}
}

func (m *HTTPManager) Init() {
	m.client = &fasthttp.Client{}

	if m.proxy != "" {

		if err := os.Setenv("HTTP_PROXY", m.proxy); err != nil {
			m.logger.Error("set http proxy error", "error", err)
		}
		if err := os.Setenv("HTTPS_PROXY", m.proxy); err != nil {
			m.logger.Error("set http proxy error", "error", err)
		}

		url := strings.TrimPrefix(m.proxy, "https://")
		url = strings.TrimPrefix(url, "http://")

		m.client.Dial = fasthttpproxy.FasthttpHTTPDialer(url)
	}
}
