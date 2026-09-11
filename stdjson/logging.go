package stdjson

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/Sn0wo2/caelum"
)

var (
	stdOut *caelum.Logger
	stdErr *caelum.Logger
)

func Init(pretty bool) {
	opts := slog.HandlerOptions{ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case slog.TimeKey:
			return slog.Attr{}
		case slog.LevelKey:
			if lv, ok := a.Value.Any().(slog.Level); ok {
				return slog.String(slog.LevelKey, strings.ToLower(lv.String()))
			}
		}
		return a
	}}
	stdOut = caelum.New(caelum.Config{
		Level: slog.LevelInfo,
		Targets: []caelum.Target{{
			Writer:         &Writer{dst: os.Stdout, pretty: pretty},
			Format:         caelum.JSON,
			HandlerOptions: opts,
		}},
	})
	stdErr = caelum.New(caelum.Config{
		Level: slog.LevelWarn,
		Targets: []caelum.Target{{
			Writer:         &Writer{dst: os.Stderr, pretty: pretty},
			Format:         caelum.JSON,
			HandlerOptions: opts,
		}},
	})
}

func Log(level slog.Level, stage, msg string, data any) {
	l := stdOut
	if level >= slog.LevelWarn {
		l = stdErr
	}
	attrs := []any{slog.String("stage", stage)}
	if data != nil {
		attrs = append(attrs, slog.Any("data", data))
	}
	l.Log(context.Background(), level, msg, attrs...)
}
