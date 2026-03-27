package configenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertEnvExample_SkipWhenMissing(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, ".env.example")
	res, err := UpsertEnvExample(InjectOptions{FilePath: p})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Skipped {
		t.Fatalf("expected skipped=true")
	}
}

func TestUpsertEnvExample_UpsertWithoutOverride(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, ".env.example")
	if err := os.WriteFile(p, []byte("APP_NAME=my-custom\nLOG_LEVEL=debug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := UpsertEnvExample(InjectOptions{FilePath: p, CommentHeader: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Updated {
		t.Fatalf("expected updated=true")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	txt := string(b)
	if !strings.Contains(txt, "APP_NAME=my-custom") {
		t.Fatalf("must preserve existing APP_NAME")
	}
	if !strings.Contains(txt, "LOG_LEVEL=debug") {
		t.Fatalf("must preserve existing LOG_LEVEL")
	}
	if !strings.Contains(txt, "OBSKIT_ELASTIC_URL=http://localhost:9200") {
		t.Fatalf("should append missing keys")
	}
}

func TestUpsertEnvExample_CreateWhenEnabled(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, ".env.example")
	res, err := UpsertEnvExample(InjectOptions{FilePath: p, CreateIfMissing: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Updated || len(res.Added) == 0 {
		t.Fatalf("expected file created with entries")
	}
}

func TestDefaultsByProfile(t *testing.T) {
	min := DefaultsByProfile(ProfileMinimal)
	loki := DefaultsByProfile(ProfileLoki)
	full := DefaultsByProfile(ProfileFull)
	if len(min) == 0 || len(full) == 0 || len(loki) == 0 {
		t.Fatalf("profiles should not be empty")
	}
	if len(min) != 9 {
		t.Fatalf("minimal profile should contain exactly 9 keys, got=%d", len(min))
	}
	if len(loki) != 5 {
		t.Fatalf("loki profile should contain exactly 5 keys, got=%d", len(loki))
	}
	if len(full) <= len(min) {
		t.Fatalf("full profile should contain more entries than minimal")
	}
}
