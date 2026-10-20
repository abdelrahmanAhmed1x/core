# Handbook examples

[handbook_test.go](handbook_test.go) contains the README snippets inside complete
Go functions, with their imports and application dependencies spelled out.
They are fragments for an existing application, not new public Core APIs.
Authentication functions take an already-verified user; none mount a bypass login.

Run from the repository root:

```sh
go test -v ./examples/handbook
```

This compiles every snippet, runs the memory-cache example, and checks that marked
Markdown snippets still match their source. It also checks local Markdown links
and anchors. It does not contact Redis, call external APIs or open a server.
The no-op OTel example needs no exporter.

Use [the runnable cache program](../cache/main.go) for a standalone first run,
[auth](../auth/main.go) for manager construction, or [server](../server/main.go)
for a full HTTP service with graceful shutdown.
