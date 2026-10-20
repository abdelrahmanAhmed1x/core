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
	"set-cookie":    {}, "api_key": {}, "apikey": {}, "signing_key": {}, "secret_key": {}, "session": {}, "session_token": {},
}

type maskingHandler struct {
	slog.Handler
	all bool
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
	if m.all {
		return slog.String(a.Key, "[REDACTED]")
	}
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, "[REDACTED]")
	}
	a.Value = a.Value.Resolve()
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
func (m *maskingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	masked := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		masked[i] = m.maskAttr(a)
	}
	return &maskingHandler{Handler: m.Handler.WithAttrs(masked), all: m.all}
}
func (m *maskingHandler) WithGroup(name string) slog.Handler {
	_, sensitive := sensitiveKeys[strings.ToLower(name)]
	return &maskingHandler{Handler: m.Handler.WithGroup(name), all: m.all || sensitive}
}
