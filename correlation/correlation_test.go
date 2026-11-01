package correlation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWithIDAndID(t *testing.T) {
	ctx := WithID(context.Background(), "corr-1")
	if got := ID(ctx); got != "corr-1" {
		t.Fatalf("expected corr-1 got=%s", got)
	}
}

func TestGetOrGenerate(t *testing.T) {
	ctx, id := GetOrGenerate(context.Background())
	if id == "" {
		t.Fatalf("expected generated id")
	}
	if got := ID(ctx); got != id {
		t.Fatalf("context id mismatch got=%s want=%s", got, id)
	}
}

func TestGenerateConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	var failed atomic.Bool
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if Generate() == "" {
				failed.Store(true)
			}
		}()
	}
	wg.Wait()
	if failed.Load() {
		t.Fatalf("generated id must not be empty")
	}
}

func TestGenerateReturnsEmptyWhenEntropyFails(t *testing.T) {
	got := generate(func([]byte) (int, error) { return 0, errors.New("entropy unavailable") })
	if got != "" {
		t.Fatalf("generate() = %q, want empty ID when entropy fails", got)
	}
}
