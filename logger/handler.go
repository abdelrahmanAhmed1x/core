package logger

import (
	"context"
	"log/slog"
	"strings"
)

var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"pass":          {},
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"secret":        {},
	"authorization": {},
	"cookie":        {},
}

type maskingHandler struct {
	slog.Handler
}

func (m *maskingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(m.maskAttr(a))
		return true
	})

	return m.Handler.Handle(ctx, newRecord)
}

func (m *maskingHandler) maskAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		masked := make([]slog.Attr, len(attrs))
		for i, gAttr := range attrs {
			masked[i] = m.maskAttr(gAttr)
		}
		return slog.Group(a.Key, convertToAnySlice(masked)...)
	}

	if _, exists := sensitiveKeys[strings.ToLower(a.Key)]; exists {
		return slog.String(a.Key, "[REDACTED]")
	}

	return a
}

func convertToAnySlice(attrs []slog.Attr) []any {
	anys := make([]any, len(attrs))
	for i, v := range attrs {
		anys[i] = v
	}
	return anys
}