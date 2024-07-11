package ws

import (
	"time"

	"github.com/dafsic/hunter/pkg/log"
)

type CallbackFunc func(msg []byte)

type Manager interface {
	NewClient(url, localIP string, cb CallbackFunc, heartbeat int, isGoroutine bool) (Client, error)
}

type WsManager struct {
	logger    log.Logger
	proxy     string
	writeWait time.Duration
}

func New(l log.Logger, opts ...Option) *WsManager {
	cfg := wsCfg{
		Proxy:     "",
		WriteWait: time.Second * 1,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &WsManager{
		logger:    l,
		proxy:     cfg.Proxy,
		writeWait: cfg.WriteWait,
	}
}
