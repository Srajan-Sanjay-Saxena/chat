# Errors & Wrapping (compact)

Principles
- Use `fmt.Errorf("...: %w", err)` to wrap errors so callers can use `errors.Is` / `errors.As`.
- Avoid comparing error strings.

Example
```go
if err := repo.CreateUser(...); errors.Is(err, repository.ErrUserExists) { /* duplicate */ }
```

JWT errors
- Wrap jwt parsing/validation errors so handlers can detect expired tokens or invalid signatures.

Sources
- Go errors package: https://pkg.go.dev/errors