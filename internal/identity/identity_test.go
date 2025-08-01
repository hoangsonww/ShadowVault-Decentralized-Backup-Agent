package identity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hoangsonww/backupagent/internal/identity"
)

func TestLoadOrCreateIdempotent(t *testing.T) {
	tmp := t.TempDir()
	priv1, pid1, err := identity.LoadOrCreate(tmp)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	priv2, pid2, err := identity.LoadOrCreate(tmp)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}
	if pid1 != pid2 {
		t.Fatalf("peer ID changed between loads: %s vs %s", pid1, pid2)
	}
	if err := identity.ValidatePeerID(priv1, pid2); err != nil {
		t.Fatalf("validate mismatch: %v", err)
	}
	if err := identity.ValidatePeerID(priv2, pid1); err != nil {
		t.Fatalf("validate mismatch: %v", err)
	}
	// ensure key file exists
	if _, err := os.Stat(filepath.Join(tmp, "identity.key")); err != nil {
		t.Fatalf("key file missing: %v", err)
	}
}
