# httpx

**What it does:** Typed Gin binding and consistent success/error JSON responses. **When to use it:** Handlers that should share request validation and response shapes.

`import "github.com/abdelrahmanAhmed1x/core/httpx"`

The example below is a fragment for an existing application. See [shared setup](../README.md#quick-start) for types and dependencies, or the [compile-checked examples](../examples/handbook/handbook_test.go) for complete imports and enclosing functions.

**How to use it:**

<!-- example:httpx -->
```go
request, ok := httpx.BindJSON[struct {
	Name string `json:"name" binding:"required"`
}](c)
if !ok {
	return
} // A 400 response has already been sent.
httpx.Created(c, Product{ID: 42, Name: request.Name})
```
<!-- /example -->

**Available API:**

| API | Purpose |
|---|---|
| `BindJSON[T] / BindURI[T] / BindQuery[T]` | Bind and validate input; return (value, ok). |
| `OK / Created / NoContent` | Send 200 / 201 / 204 responses. |
| `AbortBadRequest / AbortUnauthorized / AbortForbidden / AbortInternal` | Stop with safe HTTP errors; more Abort helpers are listed in the package README. |

**Configuration:** No constructor or config. Use Gin `json`, `uri`, `form`, and `binding` struct tags. [All options](../docs/configuration.md#httpx).

**Important notes:** Return when binding reports false: the 400 response is already sent. Keep application-supplied error messages/details safe for clients. [More](../docs/operations.md#http-responses).

## Error helpers

| API | Purpose |
|---|---|
| `AbortBadRequest(c,message,details)` | 400 |
| `AbortUnauthorized(c,message) / AbortForbidden(c,message)` | 401 / 403 |
| `AbortNotFound(c,message) / AbortMethodNotAllowed(c,message)` | 404 / 405 |
| `AbortConflict(c,message,details)` | 409 |
| `AbortUnprocessableEntity(c,message,details)` | 422 |
| `AbortTooManyRequests(c)` | 429 |
| `AbortInternal(c,err) / AbortGatewayTimeout(c,message)` | 500 / 504 |
