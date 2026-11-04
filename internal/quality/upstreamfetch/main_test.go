package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caelyreth/untie/internal/quality/fixture"
)

func TestFetchFileVerifiesAndReusesHash(t *testing.T) {
	t.Parallel()

	const content = "pinned fixture\n"
	sum := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(sum[:])
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	source := lockSource{
		Repository: server.URL,
		Commit:     strings.Repeat("a", 40),
		SourcePath: "fixture.txt",
		Target:     "fixture.txt",
	}
	target := filepath.Join(t.TempDir(), "fixture.txt")
	if err := fetchFile(context.Background(), server.Client(), source, target, expected); err != nil {
		t.Fatal(err)
	}
	if err := fetchFile(context.Background(), server.Client(), source, target, expected); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("content = %q", data)
	}
}

func TestFetchFileRejectsWrongHashWithoutReplacingTarget(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("wrong\n"))
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "fixture.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := lockSource{Repository: server.URL, Commit: strings.Repeat("a", 40), SourcePath: "fixture.txt"}
	err := fetchFile(context.Background(), server.Client(), source, target, strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("error = %v", err)
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "old\n" {
		t.Fatalf("target was replaced with %q", data)
	}
}

func TestValidateSourceRejectsTraversal(t *testing.T) {
	t.Parallel()

	base := lockSource{
		Repository: "https://github.com/example/repo",
		Commit:     strings.Repeat("a", 40),
		SourcePath: "fixture.txt",
		Target:     "fixture.txt",
	}
	if err := validateSource(base); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"../fixture.txt", "a/../../fixture.txt", "/fixture.txt"} {
		source := base
		source.Target = target
		if err := validateSource(source); err == nil {
			t.Fatalf("target %q accepted", target)
		}
	}
}

func TestTrackedSourceLockIsManifestBackedAndImageFree(t *testing.T) {
	t.Parallel()

	root, err := discoverRoot()
	if err != nil {
		t.Fatal(err)
	}
	upstreamRoot := filepath.Join(root, "testdata", "upstream")
	lock, err := loadLock(filepath.Join(upstreamRoot, "sources.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := fixture.LoadSHA256Manifest(filepath.Join(upstreamRoot, "manifest.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	listed := make(map[string]struct{}, len(manifest))
	for _, entry := range manifest {
		listed[entry.Path] = struct{}{}
	}
	for _, source := range lock.Sources {
		if _, ok := listed[source.Target]; !ok {
			t.Errorf("source target %q is absent from manifest.sha256", source.Target)
		}
		if strings.EqualFold(filepath.Ext(source.Target), ".png") {
			t.Errorf("source target %q imports an upstream raster image", source.Target)
		}
	}
}
