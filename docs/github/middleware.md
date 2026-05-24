# Middleware (compact)

- `Middleware.JWTMiddleware` centralizes bearer token verification for protected routes.
- It extracts the user id from the token, stores it in request context, and blocks unauthorized requests early.
- Protected routes now include conversation actions, message history, and websocket upgrades.