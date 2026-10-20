package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

type valuer struct{}

func (valuer) LogValue() slog.Value { return slog.GroupValue(slog.String("password", "VALUE_SECRET")) }
func TestMasking(t *testing.T) {
	var b bytes.Buffer
	l := New(Config{Output: &b, Format: "json", MaskSensitives: true})
	l.With("password", "CHILD_SECRET").Slog().WithGroup("group").Info("message", "token", "TOKEN_SECRET", "nested", valuer{}, "api_key", "KEY_SECRET")
	l.Info(context.Background(), "message", slog.Group("password", slog.String("x", "GROUP_SECRET")))
	l.Slog().WithGroup("password").With("nested", "NESTED_SECRET").Info("message", "value", "GROUP_VALUE_SECRET")
	if strings.Contains(b.String(), "SECRET") {
		t.Fatal(b.String())
	}
	if !strings.Contains(b.String(), "REDACTED") {
		t.Fatal(b.String())
	}
}
