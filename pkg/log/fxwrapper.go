package log

import (
	"context"
	"log/slog"

	"github.com/dafsic/hunter/config"

	"go.uber.org/fx"
)

const ModuleName = "log"

type Params struct {
	fx.In
	Lc fx.Lifecycle

	Cfg *config.Cfg
}

type Result struct {
	fx.Out

	Logger Logger
}

// NewFx wrap logger with fx
func NewFx(p Params) Result {
	lvl := slog.LevelInfo
	switch p.Cfg.Log.Level {
	case "debug", "DEBUG":
		lvl = slog.LevelDebug
	case "info", "INFO":
		lvl = slog.LevelInfo
	case "warn", "WARN":
		lvl = slog.LevelWarn
	case "error", "ERROR":
		lvl = slog.LevelError
	default:
	}

	l := New(WithLevel(lvl))

	p.Lc.Append(fx.Hook{
		// app.start调用
		OnStart: func(ctx context.Context) error {
			go l.Run(ctx) // 不能阻塞
			return nil
		},
		// app.stop调用，收到中断信号的时候调用app.stop
		OnStop: func(ctx context.Context) error {
			l.Stop(ctx)
			return nil
		},
	})

	return Result{Logger: l}
}

var ModuleFx = fx.Options(fx.Provide(NewFx))
