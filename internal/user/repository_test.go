package user_test

import (
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

func setupRepo(t *testing.T) (*user.Repository, *ent.Client) {
	t.Helper()
	client := testutil.NewEntClient(t, testDSN)
	repo := user.NewRepository(client)

	t.Cleanup(func() {
		testutil.CleanupEntData(context.Background(), client)
	})

	return repo, client
}

func TestCreate_Success(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	u, err := repo.Create(ctx, "alice", "hashedpass123", "alice@example.com")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if u.Username != "alice" {
		t.Errorf("Username = %q, want %q", u.Username, "alice")
	}
	if u.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", u.Email, "alice@example.com")
	}
	if u.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestCreate_DuplicateUsername(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, "bob", "hash1", "bob@example.com")
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = repo.Create(ctx, "bob", "hash2", "bob2@example.com")
	if err != user.ErrUserExists {
		t.Errorf("second Create() error = %v, want ErrUserExists", err)
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, "user1", "hash1", "same@example.com")
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = repo.Create(ctx, "user2", "hash2", "same@example.com")
	if err != user.ErrUserExists {
		t.Errorf("second Create() error = %v, want ErrUserExists", err)
	}
}

func TestGetByID_Exists(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, "charlie", "hash", "charlie@example.com")

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Username != "charlie" {
		t.Errorf("Username = %q, want %q", got.Username, "charlie")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if !ent.IsNotFound(err) {
		t.Errorf("GetByID() error = %v, want NotFound", err)
	}
}

func TestGetByUsername_Exists(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "diana", "hash", "diana@example.com")

	got, err := repo.GetByUsername(ctx, "diana")
	if err != nil {
		t.Fatalf("GetByUsername() error: %v", err)
	}
	if got.Email != "diana@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "diana@example.com")
	}
}

func TestGetByUsername_NotFound(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, err := repo.GetByUsername(ctx, "nonexistent")
	if !ent.IsNotFound(err) {
		t.Errorf("GetByUsername() error = %v, want NotFound", err)
	}
}

func TestGetByEmail_Exists(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "eve", "hash", "eve@example.com")

	got, err := repo.GetByEmail(ctx, "eve@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() error: %v", err)
	}
	if got.Username != "eve" {
		t.Errorf("Username = %q, want %q", got.Username, "eve")
	}
}

func TestGetByEmail_NotFound(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nobody@example.com")
	if !ent.IsNotFound(err) {
		t.Errorf("GetByEmail() error = %v, want NotFound", err)
	}
}

func TestSearch_ByPrefix(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "frank", "hash", "frank@example.com")
	_, _ = repo.Create(ctx, "fred", "hash", "fred@example.com")
	_, _ = repo.Create(ctx, "grace", "hash", "grace@example.com")

	results, err := repo.Search(ctx, "fr", 10)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search() returned %d results, want 2", len(results))
	}

	// Results should be ordered by username ascending
	if results[0].Username != "frank" {
		t.Errorf("results[0].Username = %q, want %q", results[0].Username, "frank")
	}
	if results[1].Username != "fred" {
		t.Errorf("results[1].Username = %q, want %q", results[1].Username, "fred")
	}
}

func TestSearch_NoMatch(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "henry", "hash", "henry@example.com")

	results, err := repo.Search(ctx, "xyz", 10)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() returned %d results, want 0", len(results))
	}
}

func TestSearch_RespectsLimit(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "test_a", "hash", "a@example.com")
	_, _ = repo.Create(ctx, "test_b", "hash", "b@example.com")
	_, _ = repo.Create(ctx, "test_c", "hash", "c@example.com")

	results, err := repo.Search(ctx, "test_", 2)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search() returned %d results, want 2 (limit)", len(results))
	}
}

func TestSearch_InvalidLimit_DefaultsTo10(t *testing.T) {
	repo, _ := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "user_x", "hash", "x@example.com")

	// Negative limit should default to 10
	results, err := repo.Search(ctx, "user_", -1)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) > 10 {
		t.Errorf("Search() returned %d results, should cap at 10 for invalid limit", len(results))
	}
}
