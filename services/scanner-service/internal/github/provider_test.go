package github

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeCache struct {
	value string
	gets  int
	sets  int
}

func (c *fakeCache) Get(context.Context, string) (string, error) {
	c.gets++
	return c.value, nil
}

func (c *fakeCache) Set(context.Context, string, string, time.Duration) error {
	c.sets++
	return nil
}

type fakeProvider struct {
	tag   string
	err   error
	calls int
}

func (p *fakeProvider) GetLatestTag(context.Context, string) (string, error) {
	p.calls++
	return p.tag, p.err
}

func TestCachedReleaseProvider_CacheHitAvoidsGitHub(t *testing.T) {
	cache := &fakeCache{value: "v1.0.0"}
	next := &fakeProvider{tag: "v2.0.0"}
	provider := NewCachedReleaseProvider(cache, next, time.Minute, nil)

	tag, err := provider.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("GetLatestTag() error = %v", err)
	}
	if tag != "v1.0.0" {
		t.Fatalf("tag = %q, want v1.0.0", tag)
	}
	if next.calls != 0 {
		t.Fatalf("next calls = %d, want 0", next.calls)
	}
}

func TestCachedReleaseProvider_CacheMissCallsGitHubAndStores(t *testing.T) {
	cache := &fakeCache{}
	next := &fakeProvider{tag: "v2.0.0"}
	provider := NewCachedReleaseProvider(cache, next, time.Minute, nil)

	tag, err := provider.GetLatestTag(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("GetLatestTag() error = %v", err)
	}
	if tag != "v2.0.0" {
		t.Fatalf("tag = %q, want v2.0.0", tag)
	}
	if next.calls != 1 || cache.sets != 1 {
		t.Fatalf("next calls/cache sets = %d/%d, want 1/1", next.calls, cache.sets)
	}
}

func TestCachedReleaseProvider_DoesNotCacheErrors(t *testing.T) {
	cache := &fakeCache{}
	next := &fakeProvider{err: errors.New("github down")}
	provider := NewCachedReleaseProvider(cache, next, time.Minute, nil)

	if _, err := provider.GetLatestTag(context.Background(), "owner/repo"); err == nil {
		t.Fatal("expected error")
	}
	if cache.sets != 0 {
		t.Fatalf("cache sets = %d, want 0", cache.sets)
	}
}
