# authorization

**What it does:** Checks roles, permissions and your own policy functions against a trusted user. **When to use it:** Admin routes, permission checks, or application resource policies.

`import "github.com/abdelrahmanAhmed1x/core/authorization"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:authorization -->
```go
access := authorization.New(func(c *gin.Context) (authorization.Principal, error) {
	user, ok := auth.Payload(c)
	if !ok {
		return authorization.Principal{}, authorization.ErrUnauthenticated
	}
	return authorization.Principal{Roles: []string{user.Role}}, nil
})
r.GET("/admin", auth.Middleware(), access.RequireRole("admin"), func(c *gin.Context) {
	c.Status(204)
})
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `New(Resolver)` | Resolve trusted roles/permissions, optionally from your database. |
| `RequireRole / RequireAnyRole / RequirePermission` | Allow matching roles or permissions. |
| `Require(Policy)` | Run your own context-aware predicate. |

**Configuration:** No config struct. A resolver returns `Principal{Roles, Permissions}` or an error. [All options](../docs/configuration.md#authorization).

**Important notes:** Missing users should return `ErrUnauthenticated` (401); denied access is 403. Empty requirements deny. Token roles may be stale: load fresh permissions when needed. [More](../docs/security.md#authorization).
