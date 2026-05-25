# Changes summary (compact)

24-05-2026 — latest edits

- Added message-history pagination controls in `handler.MessageHandler` (`before`, `limit`).
- Exposed `handler.ConvMemberListHandler` through `/conversation/members` in `main.go`.
- Added shared `handler.writeJSONError` for handler error responses.
- Added test coverage at three levels:
  - unit tests in `handler/conv_handler_test.go`
  - repository tests in `repository_test/conversation_participant_test.go`
  - end-to-end flow in `e2e_test.go`
- Split notes into GitHub-safe and internal-only indexes under `docs/`.

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
- Added a docs split index for GitHub-safe vs internal-only notes.

Why these matter:
- Centralizing auth reduces duplication and avoids accidental double verification.
- Explicit routes are simpler to reason about and easier to test.
- The new test layers cover unit, repository, and full request flow separately.