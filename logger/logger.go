package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type contextKey string

const (
	// RequestIDKey is the context key for tracing requests.
	RequestIDKey contextKey = "request_id"
	// UserIDKey is the context key for attaching authenticated user context to logs.
	UserIDKey contextKey = "user_id"
)

type Config struct {
	Level          string    // "debug", "info", "warn", "error" (Default: "info")
	Format         string    // "json" for prod, "text" or "pretty" for local dev
	Output         io.Writer // Default: os.Stdout
	MaskSensitives bool      // Mask sensitive keys like "password", "token"
}

type Logger interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	// With returns a child logger with pre-attached attributes
	With(args ...any) Logger
	// Handler returns the underlying *slog.Logger for low-level compatibility
	Slog() *slog.Logger
}

type customLogger struct {
	l *slog.Logger
}

func New(cfg Config) Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	if cfg.MaskSensitives {
		handler = &maskingHandler{Handler: handler}
	}

	return &customLogger{
		l: slog.New(handler),
	}
}

func (cl *customLogger) Slog() *slog.Logger {
	return cl.l
}

func (cl *customLogger) With(args ...any) Logger {
	return &customLogger{l: cl.l.With(args...)}
}

// logWithCtx automatically extracts trace metadata from context before printing
func (cl *customLogger) logWithCtx(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !cl.l.Enabled(ctx, level) {
		return
	}

	var attrs []any

	if ctx != nil {
		if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
			attrs = append(attrs, slog.String("request_id", reqID))
		}
		if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
			attrs = append(attrs, slog.String("user_id", userID))
		}
	}

	attrs = append(attrs, args...)
	cl.l.Log(ctx, level, msg, attrs...)
}

func (cl *customLogger) Debug(ctx context.Context, msg string, args ...any) {
	cl.logWithCtx(ctx, slog.LevelDebug, msg, args...)
}

func (cl *customLogger) Info(ctx context.Context, msg string, args ...any) {
	cl.logWithCtx(ctx, slog.LevelInfo, msg, args...)
}

func (cl *customLogger) Warn(ctx context.Context, msg string, args ...any) {
	cl.logWithCtx(ctx, slog.LevelWarn, msg, args...)
}

func (cl *customLogger) Error(ctx context.Context, msg string, args ...any) {
	cl.logWithCtx(ctx, slog.LevelError, msg, args...)
}
