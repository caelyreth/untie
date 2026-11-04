// Command upstreamfetch hydrates the raw upstream test inputs declared by the
// pinned source lock. Reviewed selections and derived fixtures remain tracked.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caelyreth/untie/internal/quality/fixture"
)

const (
	lockSchemaVersion = 1
	maxDownloadBytes  = 32 << 20
)

type lockFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	Sources       []lockSource `json:"sources"`
}

type lockSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	SourcePath string `json:"sourcePath"`
	Target     string `json:"target"`
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "upstreamfetch:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("upstreamfetch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	check := flags.Bool("check", false, "verify hydrated files without network access")
	clean := flags.Bool("clean", false, "remove hydrated files")
	root := flags.String("root", "", "repository root (default: discover from the working directory)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("usage: upstreamfetch [--check|--clean] [--root DIR]")
	}
	action := "fetch"
	if flags.NArg() == 1 {
		action = flags.Arg(0)
	}
	if *check {
		if action == "clean" {
			return errors.New("--check and --clean are mutually exclusive")
		}
		action = "check"
	}
	if *clean {
		action = "clean"
	}
	switch action {
	case "fetch", "check", "clean":
	default:
		return fmt.Errorf("unknown action %q; want fetch, check, or clean", action)
	}

	repositoryRoot := *root
	if repositoryRoot == "" {
		var err error
		repositoryRoot, err = discoverRoot()
		if err != nil {
			return err
		}
	}
	upstreamRoot := filepath.Join(repositoryRoot, "testdata", "upstream")
	lockPath := filepath.Join(upstreamRoot, "sources.lock.json")
	lock, err := loadLock(lockPath)
	if err != nil {
		return err
	}
	hashes, err := fixture.LoadSHA256Manifest(filepath.Join(upstreamRoot, "manifest.sha256"))
	if err != nil {
		return err
	}
	hashByPath := make(map[string]string, len(hashes))
	for _, entry := range hashes {
		hashByPath[entry.Path] = entry.Hash
	}
	for _, source := range lock.Sources {
		if _, ok := hashByPath[source.Target]; !ok {
			return fmt.Errorf("%s: target %q is absent from manifest.sha256", lockPath, source.Target)
		}
	}

	client := &http.Client{Timeout: 45 * time.Second}
	for _, source := range lock.Sources {
		target := filepath.Join(upstreamRoot, filepath.FromSlash(source.Target))
		switch action {
		case "clean":
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove %s: %w", source.Target, err)
			}
		case "check":
			if err := checkFile(target, hashByPath[source.Target]); err != nil {
				return fmt.Errorf("check %s: %w", source.Target, err)
			}
		case "fetch":
			if err := fetchFile(ctx, client, source, target, hashByPath[source.Target]); err != nil {
				return fmt.Errorf("fetch %s: %w", source.Target, err)
			}
			fmt.Println(source.Target)
		}
	}
	if action == "check" {
		fmt.Printf("verified %d hydrated upstream files\n", len(lock.Sources))
	}
	return nil
}

func discoverRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(directory, "testdata", "upstream", "sources.lock.json")); err == nil {
				return directory, nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("cannot find repository root containing testdata/upstream/sources.lock.json")
		}
		directory = parent
	}
}

func loadLock(path string) (lockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return lockFile{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var lock lockFile
	if err := decoder.Decode(&lock); err != nil {
		return lockFile{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := ensureEOF(decoder); err != nil {
		return lockFile{}, fmt.Errorf("%s: %w", path, err)
	}
	if lock.SchemaVersion != lockSchemaVersion {
		return lockFile{}, fmt.Errorf("%s: unsupported schema %d", path, lock.SchemaVersion)
	}
	if len(lock.Sources) == 0 {
		return lockFile{}, fmt.Errorf("%s: no sources", path)
	}
	var previous string
	seen := make(map[string]struct{}, len(lock.Sources))
	for i, source := range lock.Sources {
		if err := validateSource(source); err != nil {
			return lockFile{}, fmt.Errorf("%s: source %d: %w", path, i, err)
		}
		if previous != "" && source.Target <= previous {
			return lockFile{}, fmt.Errorf("%s: targets are not strictly sorted at %q", path, source.Target)
		}
		previous = source.Target
		if _, ok := seen[source.Target]; ok {
			return lockFile{}, fmt.Errorf("%s: duplicate target %q", path, source.Target)
		}
		seen[source.Target] = struct{}{}
	}
	return lock, nil
}

func validateSource(source lockSource) error {
	if !strings.HasPrefix(source.Repository, "https://github.com/") {
		return fmt.Errorf("repository %q is not a GitHub HTTPS URL", source.Repository)
	}
	commit, err := hex.DecodeString(source.Commit)
	if err != nil || len(commit) != 20 {
		return fmt.Errorf("commit %q is not a full SHA-1", source.Commit)
	}
	for name, value := range map[string]string{
		"sourcePath": source.SourcePath,
		"target":     source.Target,
	} {
		if value == "" || filepath.IsAbs(value) || filepath.Clean(value) != filepath.FromSlash(value) ||
			value == "." || strings.HasPrefix(value, "../") || strings.Contains(value, `/../`) {
			return fmt.Errorf("invalid %s %q", name, value)
		}
	}
	return nil
}

func fetchFile(ctx context.Context, client *http.Client, source lockSource, target, expected string) error {
	if existing, err := fileSHA256(target); err == nil && existing == expected {
		return nil
	}
	url := strings.TrimSuffix(source.Repository, "/") + "/raw/" + source.Commit + "/" + source.SourcePath
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDownloadBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxDownloadBytes {
		return fmt.Errorf("download exceeds %d bytes", maxDownloadBytes)
	}
	if err := verifyBytes(data, expected); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

func checkFile(path, expected string) error {
	actual, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("SHA-256 %s, want %s", actual, expected)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func verifyBytes(data []byte, expected string) error {
	hasher := sha256.New()
	if _, err := hasher.Write(data); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA-256 %s, want %s", actual, expected)
	}
	return nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
