package http

import (
	"context"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/pkg/log"

	"go.uber.org/fx"
)

const ModuleName = "http"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg    *config.Cfg
	Logger log.Logger
}

type Result struct {
	fx.Out

	Manager Manager
}

// NewFx wrap Manager with fx
func NewFx(p Params) Result {
	m := New(p.Logger, WithProxy(p.Cfg.Ws.Proxy))

	p.Lc.Append(fx.Hook{
		// app.start调用
		OnStart: func(ctx context.Context) error {
			m.Init() // 不能阻塞
			return nil
		},
	})

	return Result{Manager: m}
}

var ModuleFx = fx.Options(fx.Provide(NewFx))
