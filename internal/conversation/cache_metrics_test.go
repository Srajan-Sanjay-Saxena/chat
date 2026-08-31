package conversation

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	goredis "github.com/redis/go-redis/v9"

	"chat-v2/internal/metrics"
)

// newTestRedis spins up an in-memory Redis (miniredis) and returns a connected
// go-redis client. No Docker or network required.
func newTestRedis(t *testing.T) (*goredis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

// TestParticipantCache_MetricsHit verifies the cache-HIT path increments
// CacheHitsTotal (and not CacheMissesTotal). The HIT path never touches the DB,
// so a nil repo is safe here — and if the code ever wrongly fell through to the
// DB, the nil repo would panic and fail the test loudly.
//
// Metrics are global collectors, so we assert on the DELTA (after-before) to be
// independent of other tests.
func TestParticipantCache_MetricsHit(t *testing.T) {
	ctx := context.Background()
	rdb, _ := newTestRedis(t)

	convID := uuid.New()
	userID := uuid.New()

	// Seed the cache so the lookup is a guaranteed HIT.
	if err := rdb.SAdd(ctx, participantKey(convID), userID.String()).Err(); err != nil {
		t.Fatalf("seed SAdd failed: %v", err)
	}

	c := &ParticipantCache{redis: rdb, repo: nil}

	hit := metrics.CacheHitsTotal.WithLabelValues("participant")
	miss := metrics.CacheMissesTotal.WithLabelValues("participant")
	hitsBefore := testutil.ToFloat64(hit)
	missesBefore := testutil.ToFloat64(miss)

	isMember, err := c.IsParticipant(ctx, convID, userID)
	if err != nil {
		t.Fatalf("IsParticipant returned error: %v", err)
	}
	if !isMember {
		t.Fatal("expected user to be a participant (cache hit)")
	}

	if d := testutil.ToFloat64(hit) - hitsBefore; d != 1 {
		t.Errorf("CacheHitsTotal delta = %v, want 1", d)
	}
	if d := testutil.ToFloat64(miss) - missesBefore; d != 0 {
		t.Errorf("CacheMissesTotal delta = %v, want 0", d)
	}
}

// TestParticipantCache_MetricsMiss verifies the cache-MISS path increments
// CacheMissesTotal. In the current implementation the miss counter is
// incremented BEFORE the DB fallback (repo.IsParticipant) is called. Because
// building a real repo requires a live DB, we use a nil repo and recover from
// the resulting panic — the assertion is that the miss counter moved by the
// time the fallback is attempted.
func TestParticipantCache_MetricsMiss(t *testing.T) {
	ctx := context.Background()
	rdb, _ := newTestRedis(t)

	// No seed → key does not exist → Exists()==0 → clean miss branch.
	convID := uuid.New()
	userID := uuid.New()

	c := &ParticipantCache{redis: rdb, repo: nil}

	miss := metrics.CacheMissesTotal.WithLabelValues("participant")
	missBefore := testutil.ToFloat64(miss)

	func() {
		// repo is nil → DB fallback panics; recover so we can assert the
		// counter that was incremented just before the fallback.
		defer func() { _ = recover() }()
		_, _ = c.IsParticipant(ctx, convID, userID)
	}()

	if d := testutil.ToFloat64(miss) - missBefore; d != 1 {
		t.Errorf("CacheMissesTotal delta = %v, want 1", d)
	}
}
