# Middleware (compact)

What: middleware in Go is a function that wraps `http.Handler` to process requests before/after handlers.

Why: centralize cross-cutting concerns (auth, logging, CORS, rate-limiting) so handlers stay focused.

This project:
- `Middleware/jwtMiddleware.go` implements `func JWTMiddleware(maker *helper.JWTMaker) func(http.Handler) http.Handler`.
- The middleware extracts Bearer token, verifies via `helper.JWTMaker`, and injects `userID` into context with `helper.SetUserContext`.

How to use (pattern):
- Build middleware: `auth := JWTMiddleware(maker)`
- Wrap protected handlers: `mux.Handle("/conversation/join", auth(handler.ConversationJoinHandler(...)))`

Notes:
- Keep middleware small and side-effect free (limit to auth lookup + context injection).
- Prefer context-based identity for handlers to read instead of re-verifying tokens.

Further reading:
- Official Go net/http patterns: https://pkg.go.dev/net/http
- Rob Pike / Go blog on handlers/middleware patterns: https://blog.golang.org/context
- Practical middleware examples: https://www.alexedwards.net/blog/making-and-using-middleware
