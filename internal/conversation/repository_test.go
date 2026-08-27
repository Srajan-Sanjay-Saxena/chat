package conversation_test

import (
	"chat-v2/internal/conversation"
	"chat-v2/internal/domain/ent"
	"chat-v2/internal/testutil"
	"chat-v2/internal/user"
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
)

var testDSN string

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := testutil.StartPostgres(ctx)
	if err != nil {
		panic("failed to start postgres: " + err.Error())
	}

	testDSN = pg.DSN

	code := m.Run()

	pg.Container.Terminate(ctx)
	os.Exit(code)
}

type testEnv struct {
	convRepo *conversation.Repository
	userRepo *user.Repository
	client   *ent.Client
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	client := testutil.NewEntClient(t, testDSN)

	t.Cleanup(func() {
		testutil.CleanupEntData(context.Background(), client)
	})

	return &testEnv{
		convRepo: conversation.NewRepository(client),
		userRepo: user.NewRepository(client),
		client:   client,
	}
}

func createTestUser(t *testing.T, repo *user.Repository, username string) *ent.User {
	t.Helper()
	u, err := repo.Create(context.Background(), username, "hash123", username+"@example.com")
	if err != nil {
		t.Fatalf("failed to create test user %q: %v", username, err)
	}
	return u
}

// --- Create Tests ---

func TestCreate_GroupConversation(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	conv, err := env.convRepo.Create(ctx, "group", "Test Group", "", "")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if conv.Type != "group" {
		t.Errorf("Type = %q, want %q", conv.Type, "group")
	}
	if *conv.Title != "Test Group" {
		t.Errorf("Title = %q, want %q", *conv.Title, "Test Group")
	}
	if conv.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
}

func TestCreate_PrivateConversation(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	conv, err := env.convRepo.Create(ctx, "private", "", "", "alice:bob")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if conv.Type != "private" {
		t.Errorf("Type = %q, want %q", conv.Type, "private")
	}
	if *conv.CanonicalName != "alice:bob" {
		t.Errorf("CanonicalName = %q, want %q", *conv.CanonicalName, "alice:bob")
	}
}

func TestCreate_DuplicateCanonicalName(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	_, err := env.convRepo.Create(ctx, "private", "", "", "alice:bob")
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = env.convRepo.Create(ctx, "private", "", "", "alice:bob")
	if err != conversation.ErrConversationExists {
		t.Errorf("second Create() error = %v, want ErrConversationExists", err)
	}
}

// --- CreateWithParticipants Tests ---

func TestCreateWithParticipants_Success(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")

	conv, err := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateWithParticipants() error: %v", err)
	}

	if conv.Type != "private" {
		t.Errorf("Type = %q, want %q", conv.Type, "private")
	}

	// Verify participants were added
	participants, err := env.convRepo.GetParticipants(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetParticipants() error: %v", err)
	}
	if len(participants) != 2 {
		t.Errorf("participant count = %d, want 2", len(participants))
	}
}

func TestCreateWithParticipants_UserNotFound(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	createTestUser(t, env.userRepo, "alice")
	// "ghost" does not exist

	_, err := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:ghost", []string{"alice", "ghost"})
	if err != conversation.ErrUsersNotFound {
		t.Errorf("CreateWithParticipants() error = %v, want ErrUsersNotFound", err)
	}
}

func TestCreateWithParticipants_GroupWithMultiple(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")
	createTestUser(t, env.userRepo, "charlie")

	conv, err := env.convRepo.CreateWithParticipants(ctx, "group", "Team Chat", "", "", []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("CreateWithParticipants() error: %v", err)
	}

	participants, err := env.convRepo.GetParticipants(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetParticipants() error: %v", err)
	}
	if len(participants) != 3 {
		t.Errorf("participant count = %d, want 3", len(participants))
	}
}

// --- GetByID Tests ---

func TestGetByID_Exists(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	created, _ := env.convRepo.Create(ctx, "group", "My Group", "", "")

	got, err := env.convRepo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %v, want %v", got.ID, created.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	_, err := env.convRepo.GetByID(ctx, uuid.New())
	if !ent.IsNotFound(err) {
		t.Errorf("GetByID() error = %v, want NotFound", err)
	}
}

// --- GetByCanonicalName Tests ---

func TestGetByCanonicalName_Exists(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	_, _ = env.convRepo.Create(ctx, "private", "", "", "alice:bob")

	got, err := env.convRepo.GetByCanonicalName(ctx, "alice:bob")
	if err != nil {
		t.Fatalf("GetByCanonicalName() error: %v", err)
	}
	if *got.CanonicalName != "alice:bob" {
		t.Errorf("CanonicalName = %q, want %q", *got.CanonicalName, "alice:bob")
	}
}

func TestGetByCanonicalName_NotFound(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	_, err := env.convRepo.GetByCanonicalName(ctx, "nonexistent:pair")
	if !ent.IsNotFound(err) {
		t.Errorf("GetByCanonicalName() error = %v, want NotFound", err)
	}
}

// --- GetByUserID Tests ---

func TestGetByUserID_ReturnsUserConversations(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")

	_, _ = env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})
	_, _ = env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})

	convs, err := env.convRepo.GetByUserID(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetByUserID() error: %v", err)
	}
	if len(convs) != 2 {
		t.Errorf("conversation count = %d, want 2", len(convs))
	}
}

func TestGetByUserID_Empty(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	loner := createTestUser(t, env.userRepo, "loner")

	convs, err := env.convRepo.GetByUserID(ctx, loner.ID)
	if err != nil {
		t.Fatalf("GetByUserID() error: %v", err)
	}
	if len(convs) != 0 {
		t.Errorf("conversation count = %d, want 0", len(convs))
	}
}

// --- GetByUserIDWithDisplay Tests ---

func TestGetByUserIDWithDisplay_PrivateShowsOtherUsername(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")

	_, _ = env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})

	convs, err := env.convRepo.GetByUserIDWithDisplay(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetByUserIDWithDisplay() error: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("conversation count = %d, want 1", len(convs))
	}

	if convs[0].DisplayName == nil || *convs[0].DisplayName != "bob" {
		t.Errorf("DisplayName = %v, want 'bob'", convs[0].DisplayName)
	}
}

// --- IsParticipant Tests ---

func TestIsParticipant_True(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")

	conv, _ := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})

	is, err := env.convRepo.IsParticipant(ctx, conv.ID, alice.ID)
	if err != nil {
		t.Fatalf("IsParticipant() error: %v", err)
	}
	if !is {
		t.Error("IsParticipant() = false, want true")
	}
}

func TestIsParticipant_False(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")
	outsider := createTestUser(t, env.userRepo, "outsider")

	conv, _ := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})

	is, err := env.convRepo.IsParticipant(ctx, conv.ID, outsider.ID)
	if err != nil {
		t.Fatalf("IsParticipant() error: %v", err)
	}
	if is {
		t.Error("IsParticipant() = true, want false")
	}
}

// --- AddParticipant / RemoveParticipant Tests ---

func TestAddRemoveParticipant(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	bob := createTestUser(t, env.userRepo, "bob")
	charlie := createTestUser(t, env.userRepo, "charlie")

	conv, _ := env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})

	// Add charlie
	err := env.convRepo.AddParticipant(ctx, conv.ID, charlie.ID)
	if err != nil {
		t.Fatalf("AddParticipant() error: %v", err)
	}

	// Verify charlie is a participant
	is, _ := env.convRepo.IsParticipant(ctx, conv.ID, charlie.ID)
	if !is {
		t.Error("charlie should be a participant after AddParticipant")
	}

	// Remove bob
	err = env.convRepo.RemoveParticipant(ctx, conv.ID, bob.ID)
	if err != nil {
		t.Fatalf("RemoveParticipant() error: %v", err)
	}

	// Verify bob is gone
	is, _ = env.convRepo.IsParticipant(ctx, conv.ID, bob.ID)
	if is {
		t.Error("bob should not be a participant after RemoveParticipant")
	}

	// Verify alice is still there
	is, _ = env.convRepo.IsParticipant(ctx, conv.ID, alice.ID)
	if !is {
		t.Error("alice should still be a participant")
	}
}

// --- GetParticipants Tests ---

func TestGetParticipants_ReturnsAll(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	bob := createTestUser(t, env.userRepo, "bob")

	conv, _ := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})

	participants, err := env.convRepo.GetParticipants(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetParticipants() error: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("participant count = %d, want 2", len(participants))
	}

	// Check both IDs are present
	idSet := make(map[uuid.UUID]bool)
	for _, id := range participants {
		idSet[id] = true
	}
	if !idSet[alice.ID] {
		t.Error("alice ID not in participants")
	}
	if !idSet[bob.ID] {
		t.Error("bob ID not in participants")
	}
}

func TestGetParticipants_EmptyConversation(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	// Create conversation without participants
	conv, _ := env.convRepo.Create(ctx, "group", "Empty Group", "", "")

	participants, err := env.convRepo.GetParticipants(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetParticipants() error: %v", err)
	}
	if len(participants) != 0 {
		t.Errorf("participant count = %d, want 0", len(participants))
	}
}

// --- GetFriends Tests ---

func TestGetFriends_ReturnsPrivateConversationPartners(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	bob := createTestUser(t, env.userRepo, "bob")
	charlie := createTestUser(t, env.userRepo, "charlie")

	// Alice has private convs with bob and charlie
	_, _ = env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})
	_, _ = env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:charlie", []string{"alice", "charlie"})
	// Alice is also in a group with bob — this should NOT make bob appear twice
	_, _ = env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})

	friends, err := env.convRepo.GetFriends(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetFriends() error: %v", err)
	}
	if len(friends) != 2 {
		t.Fatalf("friend count = %d, want 2", len(friends))
	}

	friendSet := make(map[uuid.UUID]bool)
	for _, id := range friends {
		friendSet[id] = true
	}
	if !friendSet[bob.ID] {
		t.Error("bob should be a friend")
	}
	if !friendSet[charlie.ID] {
		t.Error("charlie should be a friend")
	}
}

func TestGetFriends_NoPrivateConversations(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")

	// Only a group conversation — no friends
	_, _ = env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})

	friends, err := env.convRepo.GetFriends(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetFriends() error: %v", err)
	}
	if len(friends) != 0 {
		t.Errorf("friend count = %d, want 0 (only group convs)", len(friends))
	}
}
