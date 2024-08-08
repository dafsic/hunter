package http

import (
	"github.com/valyala/fasthttp"
)

type HTTPClient struct {
	Req    *fasthttp.Request
	Resp   *fasthttp.Response
	Client *fasthttp.Client
}

func (h *HTTPClient) Drop() {
	fasthttp.ReleaseRequest(h.Req)
	fasthttp.ReleaseResponse(h.Resp)
}

func (h *HTTPClient) Do() error {
	return h.Client.Do(h.Req, h.Resp)
}
