package log

import (
	"log/slog"
	"os"
	"testing"
)

var loggerWithBuffer = New(os.Stderr, "log", LevelDebug)
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

func BenchmarkLoggerWithBuffer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		loggerWithBuffer.Debug("test", "index", i)
	}
}

func BenchmarkLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		logger.Debug("test", "index", i)
	}
}
