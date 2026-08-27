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

func TestCreate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		convType      string
		title         string
		canonicalName string
		wantType      string
		checkTitle    bool
		checkCanonical bool
	}{
		{
			name:       "group conversation",
			convType:   "group",
			title:      "Test Group",
			wantType:   "group",
			checkTitle: true,
		},
		{
			name:           "private conversation",
			convType:       "private",
			canonicalName:  "alice:bob",
			wantType:       "private",
			checkCanonical: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setup(t)

			conv, err := env.convRepo.Create(ctx, tt.convType, tt.title, "", tt.canonicalName)
			if err != nil {
				t.Fatalf("Create() error: %v", err)
			}

			if string(conv.Type) != tt.wantType {
				t.Errorf("Type = %q, want %q", conv.Type, tt.wantType)
			}
			if conv.ID == uuid.Nil {
				t.Error("ID should not be nil")
			}
			if tt.checkTitle && (conv.Title == nil || *conv.Title != tt.title) {
				t.Errorf("Title = %v, want %q", conv.Title, tt.title)
			}
			if tt.checkCanonical && (conv.CanonicalName == nil || *conv.CanonicalName != tt.canonicalName) {
				t.Errorf("CanonicalName = %v, want %q", conv.CanonicalName, tt.canonicalName)
			}
		})
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

func TestCreateWithParticipants(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		convType         string
		title            string
		canonicalName    string
		participants     []string
		wantCount        int
	}{
		{
			name:          "private two users",
			convType:      "private",
			canonicalName: "alice:bob",
			participants:  []string{"alice", "bob"},
			wantCount:     2,
		},
		{
			name:         "group three users",
			convType:     "group",
			title:        "Team Chat",
			participants: []string{"alice", "bob", "charlie"},
			wantCount:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setup(t)

			// Create all users
			for _, username := range tt.participants {
				createTestUser(t, env.userRepo, username)
			}

			conv, err := env.convRepo.CreateWithParticipants(ctx, tt.convType, tt.title, "", tt.canonicalName, tt.participants)
			if err != nil {
				t.Fatalf("CreateWithParticipants() error: %v", err)
			}

			participants, err := env.convRepo.GetParticipants(ctx, conv.ID)
			if err != nil {
				t.Fatalf("GetParticipants() error: %v", err)
			}
			if len(participants) != tt.wantCount {
				t.Errorf("participant count = %d, want %d", len(participants), tt.wantCount)
			}
		})
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

// --- GetByID / GetByCanonicalName Tests ---

func TestGetByID(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	created, _ := env.convRepo.Create(ctx, "group", "My Group", "", "")

	tests := []struct {
		name       string
		id         uuid.UUID
		wantErr    bool
		isNotFound bool
	}{
		{"exists", created.ID, false, false},
		{"not found", uuid.New(), true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := env.convRepo.GetByID(ctx, tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.isNotFound && !ent.IsNotFound(err) {
					t.Errorf("expected NotFound error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetByID() error: %v", err)
			}
			if got.ID != created.ID {
				t.Errorf("ID = %v, want %v", got.ID, created.ID)
			}
		})
	}
}

func TestGetByCanonicalName(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	_, _ = env.convRepo.Create(ctx, "private", "", "", "alice:bob")

	tests := []struct {
		name          string
		canonicalName string
		wantErr       bool
		isNotFound    bool
	}{
		{"exists", "alice:bob", false, false},
		{"empty canonical name", "", true, true},
		{"invalid format", "alice-bob", true, true},
		{"opposite order", "bob:alice", true, true},
		{"not found", "nonexistent:pair", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := env.convRepo.GetByCanonicalName(ctx, tt.canonicalName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.isNotFound && !ent.IsNotFound(err) {
					t.Errorf("expected NotFound error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetByCanonicalName() error: %v", err)
			}
			if *got.CanonicalName != "alice:bob" {
				t.Errorf("CanonicalName = %q, want %q", *got.CanonicalName, "alice:bob")
			}
		})
	}
}

// --- GetByUserID Tests ---

func TestGetByUserID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*testEnv) uuid.UUID // returns userID to query
		wantCount int
	}{
		{
			name: "returns user conversations",
			setup: func(env *testEnv) uuid.UUID {
				alice := createTestUser(t, env.userRepo, "alice")
				createTestUser(t, env.userRepo, "bob")
				env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})
				env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})
				return alice.ID
			},
			wantCount: 2,
		},
		{
			name: "empty for user with no conversations",
			setup: func(env *testEnv) uuid.UUID {
				loner := createTestUser(t, env.userRepo, "loner")
				return loner.ID
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setup(t)
			userID := tt.setup(env)

			convs, err := env.convRepo.GetByUserID(ctx, userID)
			if err != nil {
				t.Fatalf("GetByUserID() error: %v", err)
			}
			if len(convs) != tt.wantCount {
				t.Errorf("conversation count = %d, want %d", len(convs), tt.wantCount)
			}
		})
	}
}

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

func TestIsParticipant(t *testing.T) {
	env := setup(t)
	ctx := context.Background()

	alice := createTestUser(t, env.userRepo, "alice")
	createTestUser(t, env.userRepo, "bob")
	outsider := createTestUser(t, env.userRepo, "outsider")

	conv, _ := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})

	tests := []struct {
		name   string
		userID uuid.UUID
		want   bool
	}{
		{"participant", alice.ID, true},
		{"outsider", outsider.ID, false},
		{"nonexistent user", uuid.New(), false},
		{"nil user ID", uuid.Nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is, err := env.convRepo.IsParticipant(ctx, conv.ID, tt.userID)
			if err != nil {
				t.Fatalf("IsParticipant() error: %v", err)
			}
			if is != tt.want {
				t.Errorf("IsParticipant() = %v, want %v", is, tt.want)
			}
		})
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

func TestGetParticipants(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*testEnv) (uuid.UUID, []uuid.UUID) // returns convID and expected participant IDs
		wantCount int
	}{
		{
			name: "returns all",
			setup: func(env *testEnv) (uuid.UUID, []uuid.UUID) {
				alice := createTestUser(t, env.userRepo, "alice")
				bob := createTestUser(t, env.userRepo, "bob")
				conv, _ := env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})
				return conv.ID, []uuid.UUID{alice.ID, bob.ID}
			},
			wantCount: 2,
		},
		{
			name: "empty conversation",
			setup: func(env *testEnv) (uuid.UUID, []uuid.UUID) {
				conv, _ := env.convRepo.Create(ctx, "group", "Empty Group", "", "")
				return conv.ID, nil
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setup(t)
			convID, expectedIDs := tt.setup(env)

			participants, err := env.convRepo.GetParticipants(ctx, convID)
			if err != nil {
				t.Fatalf("GetParticipants() error: %v", err)
			}
			if len(participants) != tt.wantCount {
				t.Errorf("participant count = %d, want %d", len(participants), tt.wantCount)
			}

			// Verify expected IDs are present
			if expectedIDs != nil {
				idSet := make(map[uuid.UUID]bool)
				for _, id := range participants {
					idSet[id] = true
				}
				for _, id := range expectedIDs {
					if !idSet[id] {
						t.Errorf("expected participant %v not found", id)
					}
				}
			}
		})
	}
}

// --- GetFriends Tests ---

func TestGetFriends(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*testEnv) (uuid.UUID, []uuid.UUID) // returns userID and expected friend IDs
		wantCount int
	}{
		{
			name: "returns private conversation partners",
			setup: func(env *testEnv) (uuid.UUID, []uuid.UUID) {
				alice := createTestUser(t, env.userRepo, "alice")
				bob := createTestUser(t, env.userRepo, "bob")
				charlie := createTestUser(t, env.userRepo, "charlie")

				env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:bob", []string{"alice", "bob"})
				env.convRepo.CreateWithParticipants(ctx, "private", "", "", "alice:charlie", []string{"alice", "charlie"})
				// Group conv shouldn't affect friends
				env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})

				return alice.ID, []uuid.UUID{bob.ID, charlie.ID}
			},
			wantCount: 2,
		},
		{
			name: "no private conversations",
			setup: func(env *testEnv) (uuid.UUID, []uuid.UUID) {
				alice := createTestUser(t, env.userRepo, "alice")
				createTestUser(t, env.userRepo, "bob")
				env.convRepo.CreateWithParticipants(ctx, "group", "Team", "", "", []string{"alice", "bob"})
				return alice.ID, nil
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setup(t)
			userID, expectedFriends := tt.setup(env)

			friends, err := env.convRepo.GetFriends(ctx, userID)
			if err != nil {
				t.Fatalf("GetFriends() error: %v", err)
			}
			if len(friends) != tt.wantCount {
				t.Errorf("friend count = %d, want %d", len(friends), tt.wantCount)
			}

			// Verify expected friends are present
			if expectedFriends != nil {
				friendSet := make(map[uuid.UUID]bool)
				for _, id := range friends {
					friendSet[id] = true
				}
				for _, id := range expectedFriends {
					if !friendSet[id] {
						t.Errorf("expected friend %v not found", id)
					}
				}
			}
		})
	}
}
