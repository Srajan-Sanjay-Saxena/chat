# Task List

- [x] Add `SearchUsers` repo method and `UserSearchHandler` endpoint
- [x] Add `canonical_name` column + migration and create unique partial index for private DMs
- [x] Add `GetConversationByCanonicalName` repo method
- [x] Update `ConvCreateHandler` to accept participant usernames, resolve to IDs, derive `canonical_name` for private chats, and deduplicate
- [x] Keep backward compatibility: accept participant IDs and usernames; default behavior unchanged for group chats
- [x] Update repository `CreateConversationWithParticipants` and other query signatures to handle `canonical_name`
- [x] Update `ConvListHandler` to return `display_name` and `canonical_name` fields for client
- [x] Add unit and e2e tests for username-based create/search flow and deduplication
- [x] Add rate-limiting and input validation for user-search endpoint use builtin package
- [x] Compute private chat `display_name` server-side per requesting user; remove persisted `display_name` column

Notes:
- I updated the repository and migrations so tests can run against the new schema.
- The create conversation API is now username-only for new clients.
- Conversation creation now uses one repo call for usernames instead of fetching each participant individually.


For conv create handler for using names we should create another repo method instead of getting id for each user it is so many calls