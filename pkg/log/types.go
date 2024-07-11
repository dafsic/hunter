package log

import "log/slog"

type Message struct {
	Level slog.Level
	Msg   string
	Args  []any
}

func NewMessage() any {
	return new(Message)
}
