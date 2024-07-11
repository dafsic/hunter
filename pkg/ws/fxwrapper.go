package ws

import (
	"time"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/pkg/log"

	"go.uber.org/fx"
)

const ModuleName = "ws"

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
	writeWait := p.Cfg.Ws.WriteWait
	m := New(p.Logger, WithProxy(p.Cfg.Ws.Proxy), WithWriteWait(time.Duration(writeWait)*time.Second))

	return Result{Manager: m}
}

var ModuleFx = fx.Options(fx.Provide(NewFx))
