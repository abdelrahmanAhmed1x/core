# auth/stateless

**What it does:** Signed access and refresh JWTs without a session store. **When to use it:** Services that accept reusable refresh credentials and no individual server-side revocation.

`import "github.com/abdelrahmanAhmed1x/core/auth/stateless"`

The example below is a fragment for an existing application. See [shared setup](../../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:stateless -->
```go
auth := stateless.New[User](stateless.Config{
	Issuer: "shop", AccessAudience: "api", RefreshAudience: "refresh",
	AccessKey: signingKey, RefreshKey: refreshKey,
	AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
})
pair, err := auth.Issue(verifiedUser)
if err != nil {
	return err
}
user, err := auth.VerifyAccess(pair.Access)
if err != nil {
	return err
}
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New[T](Config)` | Create the JWT pair manager. |
| `Issue` | Return Pair{Access, Refresh} after identity verification. |
| `VerifyAccess / VerifyRefresh` | Check the intended credential and return its payload. |
| `Refresh` | Issue another pair from a valid refresh JWT. |
| `Middleware / OptionalMiddleware / Payload` | Protect access routes / allow guests / read the user. |

**Configuration:** `Issuer`, distinct `AccessAudience`/`RefreshAudience`, distinct `AccessKey`/`RefreshKey`, and both TTLs are required. `Leeway=0`; `Transport` controls access cookies. [All options](../../docs/configuration.md#auth-stateless).

**Important notes:** **Neither JWT can be individually revoked before expiry. Refreshing does not invalidate the old pair.** Delete client credentials to sign out locally; there is no server-side Logout. [More](../../docs/authentication.md#stateless-pair).

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

## Refresh a pair

<!-- example:stateless-refresh -->
```go
nextPair, err := auth.Refresh(pair.Refresh)
if err != nil {
	return err
}
```
<!-- /example -->
