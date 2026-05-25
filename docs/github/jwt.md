# JWT Notes (compact)

- `helper.JWTMaker` creates and verifies HS256 JWTs.
- `helper.JWTVerifier` reads the bearer token, validates it, and returns the user id.
- `helper.SetUserContext` and `helper.GetUserFromContext` pass identity through the request chain.