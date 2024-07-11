package log

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

type Logger interface {
	Log(level slog.Level, msg string, args ...any)
}

type BufferLogger struct {
	buffer chan *Message
	logger *slog.Logger
	level  slog.Level
	stopC  chan struct{}
	pool   *sync.Pool
}

// New 创建一个新的BufferLogger，实现了Logger接口
func New(opts ...Option) *BufferLogger {
	cfg := logCfg{
		Out:   os.Stdout,
		Level: slog.LevelInfo,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	handler := slog.NewTextHandler(cfg.Out, &slog.HandlerOptions{
		Level: cfg.Level,
	})

	return &BufferLogger{
		buffer: make(chan *Message, 2000),
		logger: slog.New(handler),
		level:  cfg.Level,
		stopC:  make(chan struct{}),
		pool: &sync.Pool{
			New: NewMessage,
		},
	}
}

func (b *BufferLogger) Log(level slog.Level, msg string, args ...any) {
	if level < b.level {
		return
	}

	m := (b.pool.Get()).(*Message)
	m.Level = level
	m.Msg = msg
	m.Args = args

	// 缓冲区要足够大，否则可能会阻塞
	b.buffer <- m

	// select {
	// case b.buffer <- m:
	// case <-time.After(1 * time.Second):
	// }

}

func (b *BufferLogger) Run(_ context.Context) {
	var (
		msg *Message
		ok  bool
	)

	defer func() {
		b.stopC <- struct{}{}
	}()

	for {
		select {
		case msg, ok = <-b.buffer:
			if !ok {
				return
			}
			switch msg.Level {
			case slog.LevelDebug:
				b.logger.Debug(msg.Msg, msg.Args...)
			case slog.LevelInfo:
				b.logger.Info(msg.Msg, msg.Args...)
			case slog.LevelWarn:
				b.logger.Warn(msg.Msg, msg.Args...)
			case slog.LevelError:
				b.logger.Error(msg.Msg, msg.Args...)
			default:
				b.logger.Info(msg.Msg, msg.Args...)
			}
			b.pool.Put(msg)
		}
	}
}

func (b *BufferLogger) Stop(_ context.Context) {
	close(b.buffer)
	<-b.stopC
}
