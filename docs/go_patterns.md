# Go Patterns — Context, Concurrency, HTTP (compact)

Context & concurrency
- Use `context.Context` as first param in DB/IO functions; derive with `WithTimeout`, `WithCancel`, `WithDeadline`.
- Goroutines + `sync.WaitGroup` for background work; prefer channels for coordination.

HTTP
- Build `mux := http.NewServeMux()`; register handlers with `Handle`/`HandleFunc`.
- Middleware pattern: `func Middleware(next http.Handler) http.Handler` — wrap for auth, logging, CORS.

Env
- `github.com/joho/godotenv` for `.env` loading in local dev.

Snippets
- Context timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```
- Start server:
```go
mux := http.NewServeMux()
http.ListenAndServe(":8080", mux)
```

Sources
- Official Go blog on context: https://blog.golang.org/context
