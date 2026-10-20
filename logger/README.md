# logger

**What it does:** Contextual structured logging backed by standard slog. **When to use it:** JSON service logs and request-correlated Gin access logs.

`import "github.com/abdelrahmanAhmed1x/core/logger"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:logger -->
```go
log := logger.New(logger.Config{Format: "json", MaskSensitives: true})
r.Use(logger.Middleware(log))
log.Info(ctx, "service started", "component", "api")
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(Config)` | Create a logger. |
| `Debug / Info / Warn / Error / With` | Log with context and structured attributes / attach shared attributes. |
| `Middleware / ContextWithRequestID / Slog` | Access logs / correlation context / standard slog integration. |

**Configuration:** `Level=info`, `Format=text`, `Output=stdout`. Set `Format:"json"` and `MaskSensitives:true` to enable known-key redaction. [All options](../docs/configuration.md#logger).

**Important notes:** Redaction matches attribute keys; it does not scrub arbitrary messages or error strings. Never put credentials there. [More](../docs/security.md#logging).
