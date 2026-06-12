package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"subber/pkg/contracts"
)

type fakeStore struct {
	claimed      []WatchedRepo
	claimLimit   int
	nextScanIn   time.Duration
	startedRepo  string
	stoppedRepo  string
	detectedRepo string
	detectedTag  string
	published    bool
	err          error
}

func (s *fakeStore) ClaimDue(_ context.Context, limit int, nextScanIn time.Duration) ([]WatchedRepo, error) {
	s.claimLimit = limit
	s.nextScanIn = nextScanIn
	return s.claimed, s.err
}

func (s *fakeStore) StartWatching(_ context.Context, repo string) error {
	s.startedRepo = repo
	return s.err
}

func (s *fakeStore) StopWatching(_ context.Context, repo string) error {
	s.stoppedRepo = repo
	return s.err
}

func (s *fakeStore) MarkReleaseDetected(_ context.Context, repo, tag, _ string) (bool, error) {
	s.detectedRepo = repo
	s.detectedTag = tag
	return s.published, s.err
}

type fakeReleaseProvider struct {
	tag string
	err error
}

func (p fakeReleaseProvider) GetLatestTag(context.Context, string) (string, error) {
	return p.tag, p.err
}

func TestHandleWatchlistEvent_StartAndStop(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, fakeReleaseProvider{}, nil, 10, time.Minute)

	start := contracts.Envelope[contracts.RepoWatchPayload]{
		EventType: contracts.EventRepoWatchStart,
		Payload:   contracts.RepoWatchPayload{Repo: "owner/repo"},
	}
	startRaw, _ := json.Marshal(start)
	if err := svc.HandleWatchlistEvent(context.Background(), startRaw); err != nil {
		t.Fatalf("HandleWatchlistEvent(start) error = %v", err)
	}
	if store.startedRepo != "owner/repo" {
		t.Fatalf("startedRepo = %q, want owner/repo", store.startedRepo)
	}

	stop := contracts.Envelope[contracts.RepoWatchPayload]{
		EventType: contracts.EventRepoWatchStop,
		Payload:   contracts.RepoWatchPayload{Repo: "owner/repo"},
	}
	stopRaw, _ := json.Marshal(stop)
	if err := svc.HandleWatchlistEvent(context.Background(), stopRaw); err != nil {
		t.Fatalf("HandleWatchlistEvent(stop) error = %v", err)
	}
	if store.stoppedRepo != "owner/repo" {
		t.Fatalf("stoppedRepo = %q, want owner/repo", store.stoppedRepo)
	}
}

func TestHandleWatchlistEvent_ReturnsDecodeUnsupportedAndStoreErrors(t *testing.T) {
	svc := NewService(&fakeStore{}, fakeReleaseProvider{}, nil, 10, time.Minute)
	if err := svc.HandleWatchlistEvent(context.Background(), []byte("not-json")); err == nil {
		t.Fatal("expected decode error, got nil")
	}

	unsupported := contracts.Envelope[contracts.RepoWatchPayload]{
		EventType: "UnknownEvent",
		Payload:   contracts.RepoWatchPayload{Repo: "owner/repo"},
	}
	rawUnsupported, _ := json.Marshal(unsupported)
	if err := svc.HandleWatchlistEvent(context.Background(), rawUnsupported); err == nil {
		t.Fatal("expected unsupported event error, got nil")
	}

	storeErr := errors.New("db down")
	svc = NewService(&fakeStore{err: storeErr}, fakeReleaseProvider{}, nil, 10, time.Minute)
	start := contracts.Envelope[contracts.RepoWatchPayload]{
		EventType: contracts.EventRepoWatchStart,
		Payload:   contracts.RepoWatchPayload{Repo: "owner/repo"},
	}
	rawStart, _ := json.Marshal(start)
	if err := svc.HandleWatchlistEvent(context.Background(), rawStart); !errors.Is(err, storeErr) {
		t.Fatalf("store error = %v, want %v", err, storeErr)
	}
}

func TestScanOnce_ReturnsClaimError(t *testing.T) {
	claimErr := errors.New("claim failed")
	svc := NewService(&fakeStore{err: claimErr}, fakeReleaseProvider{}, nil, 10, time.Minute)
	if err := svc.ScanOnce(context.Background()); !errors.Is(err, claimErr) {
		t.Fatalf("ScanOnce() error = %v, want %v", err, claimErr)
	}
}

func TestScanOnce_ClaimsConfiguredBatchAndPublishesNewRelease(t *testing.T) {
	store := &fakeStore{
		claimed:   []WatchedRepo{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}},
		published: true,
	}
	svc := NewService(store, fakeReleaseProvider{tag: "v2.0.0"}, nil, 25, 45*time.Second)

	if err := svc.ScanOnce(context.Background()); err != nil {
		t.Fatalf("ScanOnce() error = %v", err)
	}
	if store.claimLimit != 25 || store.nextScanIn != 45*time.Second {
		t.Fatalf("claim args = (%d, %s), want (25, 45s)", store.claimLimit, store.nextScanIn)
	}
	if store.detectedRepo != "owner/repo" || store.detectedTag != "v2.0.0" {
		t.Fatalf("detected = (%q, %q), want (owner/repo, v2.0.0)", store.detectedRepo, store.detectedTag)
	}
}

func TestScanOnce_DoesNotPublishUnchangedOrErroredTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		err  error
	}{
		{"unchanged", "v1.0.0", nil},
		{"empty", "", nil},
		{"error", "", errors.New("github down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{claimed: []WatchedRepo{{Repo: "owner/repo", LastSeenTag: "v1.0.0"}}}
			svc := NewService(store, fakeReleaseProvider{tag: tt.tag, err: tt.err}, nil, 10, time.Minute)
			if err := svc.ScanOnce(context.Background()); err != nil {
				t.Fatalf("ScanOnce() error = %v", err)
			}
			if store.detectedRepo != "" {
				t.Fatalf("release should not be marked, got repo %q", store.detectedRepo)
			}
		})
	}
}
