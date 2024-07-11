package binance

import (
	"context"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/pkg/http"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"

	"go.uber.org/fx"
)

const ModuleName = "binance"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg    *config.Cfg
	Logger log.Logger
	WS     ws.Manager
	HTTP   http.Manager
}

type Result struct {
	fx.Out

	Exchange *BinanceSpotExchange `name:"BinanceSpot"`
}

// NewFx wrap NewBinanceSpot with fx
func NewFx(p Params) (Result, error) {
	e, err := NewBinanceSpot(p.Logger, p.WS, p.HTTP, p.Cfg.Binance)
	if err != nil {
		return Result{}, err
	}

	p.Lc.Append(fx.Hook{
		// app.start调用
		OnStart: func(ctx context.Context) error {
			e.Init() // 不能阻塞
			return nil
		},
		// app.stop调用，收到中断信号的时候调用app.stop
		OnStop: func(ctx context.Context) error {
			e.Exit()
			return nil
		},
	})

	return Result{Exchange: e}, nil
}

var ModuleFx = fx.Options(fx.Provide(NewFx))
