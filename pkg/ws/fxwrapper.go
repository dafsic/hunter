package ws

import (
	"context"
	"os"
	"time"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/pkg/log"

	"go.uber.org/fx"
)

const ModuleName = "ws"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg *config.Cfg
}

type Result struct {
	fx.Out

	Manager Manager
}

// NewFx wrap Manager with fx
func NewFx(p Params) Result {
	logger := log.New(os.Stdout, ModuleName, log.StringToLevel(p.Cfg.Log.Level))
	writeWait := p.Cfg.Ws.WriteWait
	m := New(logger, WithProxy(p.Cfg.Ws.Proxy), WithWriteWait(time.Duration(writeWait)*time.Second))

	p.Lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Flush()
			return nil
		},
	})

	return Result{Manager: m}
}

var ModuleFx = fx.Options(fx.Provide(NewFx))
