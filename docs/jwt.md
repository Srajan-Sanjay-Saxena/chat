# JWT (compact)

Overview:
- `helper/jwt_maker.go` provides `JWTMaker` with `CreateToken` and `VerifyToken`.
- Claims include `RegisteredClaims` + `ID uuid.UUID` (subject matches `ID`).

Key helpers in repo:
- `ExtractBearerToken(r)` — parses `Authorization: Bearer <token>` header.
- `JWTVerifier(r, maker)` — convenience: first checks `helper.GetUserFromContext`, falls back to extracting and verifying the token and returns `uuid`.
- `SetUserContext` / `GetUserFromContext` — context helpers used by middleware and handlers.

Design notes:
- Tokens are HS256-signed using `secretKey` (min 32 chars). Keep secret out of repo/env secure.
- `VerifyToken` validates signature, subject==ID, issuer == `chat-v2` and non-nil ID.

Further reading:
- JWT RFC: https://datatracker.ietf.org/doc/html/rfc7519
- jwt-go (v5) docs: https://pkg.go.dev/github.com/golang-jwt/jwt/v5
