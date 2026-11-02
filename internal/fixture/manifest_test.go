package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedSHA256Manifest(t *testing.T) {
	t.Parallel()
	root := upstreamRoot(t)
	if err := VerifySHA256Manifest(root, filepath.Join(root, "manifest.sha256")); err != nil {
		t.Fatal(err)
	}
}

func TestSHA256ManifestRejectsMismatchAndUnlistedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, err := SHA256File(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "manifest.sha256")
	if err := os.WriteFile(manifest, []byte(hash+"  a.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256Manifest(root, manifest); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256Manifest(root, manifest); err == nil || !strings.Contains(err.Error(), "unlisted") {
		t.Fatalf("unlisted file error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256Manifest(root, manifest); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestSHA256ManifestRejectsDuplicateAndTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := filepath.Join(root, "manifest.sha256")
	hash := strings.Repeat("0", 64)
	for _, text := range []string{
		hash + "  ../outside\n",
		hash + "  a\n" + hash + "  a\n",
	} {
		if err := os.WriteFile(manifest, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSHA256Manifest(manifest); err == nil {
			t.Fatalf("manifest %q unexpectedly accepted", text)
		}
	}
}
