package spot

import (
	"context"
	"os"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange"
	"github.com/dafsic/hunter/pkg/http"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"

	"go.uber.org/fx"
)

const ModuleName = "binance.spot"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg  *config.Cfg
	WS   ws.Manager
	HTTP http.Manager
}

type Result struct {
	fx.Out

	Exchange exchange.Exchange `name:"BinanceSpot"`
}

// NewFx wrap NewBinanceSpot with fx
func NewFx(p Params) (Result, error) {
	l := log.New(os.Stdout, ModuleName, log.StringToLevel(p.Cfg.Log.Level))
	e, err := NewBinanceSpot(l, p.WS, p.HTTP, p.Cfg.Binance)
	if err != nil {
		return Result{}, err
	}

	p.Lc.Append(fx.Hook{
		// app.start调用
		OnStart: func(ctx context.Context) error {
			return e.Init() // 不能阻塞
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
