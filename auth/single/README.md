# auth/single

**What it does:** One expiring JWT, with no refresh token or datastore. **When to use it:** Small APIs and short-lived credentials where server-side revocation is unnecessary.

`import "github.com/abdelrahmanAhmed1x/core/auth/single"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:single -->
```go
auth := single.New[User](single.Config{
	SecretKey: signingKey, Issuer: "shop", Audience: "api", TTL: 15 * time.Minute,
})
token, err := auth.Issue(verifiedUser)
if err != nil {
	return err
}
user, err := auth.Verify(token)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](Config)` | Create the JWT manager. |
| `Issue / Verify` | Sign a credential / check it and return the payload. |
| `Middleware / OptionalMiddleware / Payload` | Require login / allow guests / read the typed user. |

**Configuration:** `SecretKey`, `Issuer`, `Audience`, and `TTL` are required. `Leeway=0`; `Transport` controls cookies. [All options](../../docs/configuration.md#auth-single).

**Important notes:** **A copied JWT remains valid until expiry**, even after local cookie deletion. There is no refresh or server-side Logout. [More](../../docs/authentication.md#single-jwt).

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
