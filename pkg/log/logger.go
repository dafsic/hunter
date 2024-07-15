package log

import (
	"io"
	"log/slog"

	"github.com/dafsic/hunter/utils"
)

type BufferLogger struct {
	logger *slog.Logger
	bw     *utils.BufferedWriter
}

func New(out io.Writer, prefix string, level Level) *BufferLogger {
	bufferedWriter := utils.NewBufferedWriter(out, 2048) // 2KB
	textHandler := slog.NewTextHandler(bufferedWriter, &slog.HandlerOptions{
		Level: slog.Level(level.toSlogLevel()),
	})
	logger := slog.New(textHandler).With("prefix", prefix)

	return &BufferLogger{
		logger: logger,
		bw:     bufferedWriter,
	}
}

func (l *BufferLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, args...)
}

func (l *BufferLogger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, args...)
}

func (l *BufferLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, args...)
}

func (l *BufferLogger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, args...)
}

func (l *BufferLogger) Flush() error {
	return l.bw.Flush()
}
