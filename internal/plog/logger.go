package plog

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dusted-go/logging/prettylog"
)

var (
	mux      sync.RWMutex
	logLevel = slog.LevelInfo
)

func SetLevel(level slog.Level) {
	mux.Lock()
	defer mux.Unlock()
	logLevel = level
}

func GetLevel() slog.Level {
	mux.RLock()
	defer mux.RUnlock()
	return logLevel
}

func Debug() func() {
	prev := GetLevel()
	SetLevel(slog.LevelDebug)
	return func() { SetLevel(prev) }
}

type handler struct {
	slog.Handler
	prefix string
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	record.Message = h.prefix + record.Message
	return h.Handler.Handle(ctx, record)
}

func New(name string) *slog.Logger {
	mux.RLock()
	defer mux.RUnlock()

	var prefix string
	if name != "" {
		prefix = "[" + name + "] "
	}

	return slog.New(&handler{
		Handler: prettylog.NewHandler(&slog.HandlerOptions{
			Level:       logLevel,
			AddSource:   false,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { return a },
		}),
		prefix: prefix,
	})
}
