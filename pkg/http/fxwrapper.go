package http

import (
	"context"
	"os"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/pkg/log"

	"go.uber.org/fx"
)

const ModuleName = "http"

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
	m := New(logger, WithProxy(p.Cfg.Ws.Proxy))

	p.Lc.Append(fx.Hook{
		// app.start调用
		OnStart: func(ctx context.Context) error {
			m.Init() // 不能阻塞
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Flush()
			return nil
		},
	})

	return Result{Manager: m}
}

var ModuleFx = fx.Options(fx.Provide(NewFx))
