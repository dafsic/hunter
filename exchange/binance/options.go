package binance

import "github.com/dafsic/hunter/pkg/log"

type spotCfg struct {
	logger *log.Logger
}

type Option func(*spotCfg)
