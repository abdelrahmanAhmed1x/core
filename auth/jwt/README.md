# auth/jwt

**What it does:** JWT access tokens with opaque refresh credentials stored in Redis/DragonflyDB. **When to use it:** Applications that need JWT access tokens and persistent login.

`import hybrid "github.com/abdelrahmanAhmed1x/core/auth/jwt"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:hybrid -->
```go
auth := hybrid.New[User](rdb, hybrid.Config{
	SecretKey: signingKey, Issuer: "shop", CookieSecure: true,
	AllowedOrigins: []string{"https://app.example.com"},
})
if err := auth.Login(c, verifiedUser); err != nil {
	return err
}
// In separate handlers: auth.Refresh(c) and auth.Logout(c), both return error.
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](client, Config)` | Create the hybrid Authenticator[T]. |
| `Login / Refresh / Logout` | Set credential cookies / refresh the pair / revoke refresh state and clear cookies. |
| `Middleware / OptionalMiddleware` | Verify access credentials / optionally authenticate. |
| `Payload / MustPayload` | Read the typed user; MustPayload panics if missing. |

**Configuration:** `SecretKey` and `Issuer` are required. `AccessTTL=15m`, `RefreshTTL=7d`; `Audience` is optional. Set `CookieSecure:true` and trusted `AllowedOrigins`. [All options](../../docs/configuration.md#auth-jwt).

**Important notes:** Logout revokes refresh state; existing access JWTs still work until expiry. This package’s operations use Gin contexts. Its optional middleware treats invalid single credentials as guests. [More](../../docs/authentication.md#hybrid-jwt).

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
