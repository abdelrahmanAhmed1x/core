# auth/stateful

**What it does:** Opaque login sessions stored in Redis or DragonflyDB. Sessions support logout and revocation. **When to use it:** Web/mobile applications where users need device logout or logout-all.

`import "github.com/abdelrahmanAhmed1x/core/auth/stateful"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:stateful -->
```go
auth := stateful.New[User](rdb, stateful.Config{
	Namespace: "shop:sessions", AbsoluteTTL: 24 * time.Hour,
})
token, err := auth.Create(ctx, "user-42", verifiedUser)
if err != nil {
	return err
}
user, err := auth.Verify(ctx, token)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](client, Config)` | Create a session manager. |
| `Create / Verify` | Issue a session credential / look it up. |
| `Renew / Revoke / RevokeAll` | Extend idle lifetime / revoke one session / revoke a user’s sessions. |
| `Login / Logout` | Set or clear browser credentials and update stored sessions. |
| `Middleware / OptionalMiddleware / Payload` | Require login / allow guests / read the typed user. |

**Configuration:** `Namespace` and `AbsoluteTTL` are required. `IdleTTL=0` disables idle expiry; `OperationTimeout` defaults to `3s`. `Transport` controls cookies. [All options](../../docs/configuration.md#auth-stateful).

**Important notes:** Protected requests fail when Redis is unavailable. Renewal never extends the original maximum session age. For browser writes, set `Transport.AllowedOrigins`; send bearer credentials from native clients. [More](../../docs/authentication.md#stateful-sessions).

## Gin routes and authenticated users

The same route pattern works with this manager:

<!-- example:auth-route -->
```go
r.GET("/me", auth.Middleware(), func(c *gin.Context) {
	user, ok := auth.Payload(c)
	if !ok {
		c.AbortWithStatus(401)
		return
	}
	c.JSON(200, user)
})
```
<!-- /example -->

## Browser login and session actions

<!-- example:stateful-web -->
```go
// After password/MFA verification in your existing login handler:
if err := auth.Login(c, "user-42", verifiedUser); err != nil {
	return err
}
// In your logout handler, return auth.Logout(c).
```
<!-- /example -->

<!-- example:stateful-actions -->
```go
if _, err := auth.Renew(ctx, token); err != nil {
	return err
}
if err := auth.Revoke(ctx, token); err != nil {
	return err
}
return auth.RevokeAll(ctx, "user-42")
```
<!-- /example -->
