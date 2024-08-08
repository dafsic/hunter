package test

import (
	"context"
	"os"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/pkg/log"

	"go.uber.org/fx"
)

const ModuleName = "test"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg         *config.Cfg
	BinanceSpot exchange.Exchange `name:"BinanceSpot"`
}

type Result struct {
	fx.Out

	Test Stratety
}

func NewFx(p Params) (Result, error) {
	logger := log.New(os.Stdout, ModuleName, log.LevelDebug)

	t, err := NewTest(logger, p.BinanceSpot, p.Cfg.Binance)
	if err != nil {
		return Result{}, err
	}

	p.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go t.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Flush()
			return nil
		},
	})

	return Result{Test: t}, nil
}

// Module for fx
var ModuleFx = fx.Module(ModuleName,
	fx.Provide(NewFx),
	fx.Invoke(Stratety.Active),
)
