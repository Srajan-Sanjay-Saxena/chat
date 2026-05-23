# Changes summary (compact)

23-05-2026 — key edits

- Added `Middleware/jwtMiddleware.go` — centralized JWT auth.
- Updated `helper/jwt_maker.go` — added `JWTVerifier`, fixed token parsing and context helpers.
- Split conversation operations into explicit handlers:
  - `handler.ConversationJoinHandler`
  - `handler.ConversationLeaveHandler`
  - `handler.conversationMembershipHandler` (shared logic)
- Wired middleware in `main.go` for protected routes: `/conversation/*`, `/past_messages`, `/ws`.
- Added middleware unit test: `Middleware/jwtMiddleware_test.go` (validates protected endpoint access).
- Documented changes into `docs/` and added an index in `chat_notes.md`.

Why these matter:
- Centralizing auth reduces duplication and avoids accidental double verification.
- Explicit routes are simpler to reason about and easier to test.
