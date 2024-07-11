package test

import (
	"context"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange/binance"
	"github.com/dafsic/hunter/pkg/log"

	"go.uber.org/fx"
)

const ModuleName = "test"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg         *config.Cfg
	Logger      log.Logger
	BinanceSpot *binance.BinanceSpotExchange `name:"BinanceSpot"`
}

type Result struct {
	fx.Out

	Test Stratety
}

func NewFx(p Params) Result {

	t := NewTest(p.Logger, p.BinanceSpot)

	p.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go t.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})

	return Result{Test: t}
}

// Module for fx
var ModuleFx = fx.Module(ModuleName,
	fx.Provide(NewFx),
	fx.Invoke(Stratety.Active),
)
