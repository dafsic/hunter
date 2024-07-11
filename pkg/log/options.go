package log

import (
	"io"
	"log/slog"
)

type logCfg struct {
	Out   io.Writer
	Level slog.Level
}

type Option func(*logCfg)

func WithOut(out io.Writer) Option {
	return func(o *logCfg) {
		o.Out = out
	}
}

func WithLevel(level slog.Level) Option {
	return func(o *logCfg) {
		o.Level = level
	}
}
