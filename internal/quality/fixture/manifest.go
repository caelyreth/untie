package fixture

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// ManifestEntry is one SHA-256 manifest row.
type ManifestEntry struct {
	Hash string
	Path string
}

// SHA256File returns the lowercase hexadecimal SHA-256 of a file.
func SHA256File(filePath string) (checksum string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// LoadSHA256Manifest decodes a strict "hash  relative/path" manifest.
func LoadSHA256Manifest(filePath string) (entries []ManifestEntry, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	var parsed []ManifestEntry
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			return nil, fmt.Errorf("%s:%d: blank manifest line", filePath, line)
		}
		fields := strings.Fields(text)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s:%d: want SHA-256 and path", filePath, line)
		}
		if len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("%s:%d: invalid SHA-256 length", filePath, line)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return nil, fmt.Errorf("%s:%d: invalid SHA-256: %w", filePath, line, err)
		}
		rel := path.Clean(fields[1])
		if rel != fields[1] || !safeRelativePath(rel) {
			return nil, fmt.Errorf("%s:%d: invalid fixture path %q", filePath, line, fields[1])
		}
		if _, ok := seen[rel]; ok {
			return nil, fmt.Errorf("%s:%d: duplicate path %q", filePath, line, rel)
		}
		seen[rel] = struct{}{}
		parsed = append(parsed, ManifestEntry{Hash: strings.ToLower(fields[0]), Path: rel})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(parsed) == 0 {
		return nil, errors.New("empty SHA-256 manifest")
	}
	return parsed, nil
}

// VerifySHA256Manifest verifies every listed file and rejects unlisted files.
// The manifest itself is the only excluded path below root.
func VerifySHA256Manifest(root, manifestFile string) error {
	entries, err := LoadSHA256Manifest(manifestFile)
	if err != nil {
		return err
	}
	manifestAbs, err := filepath.Abs(manifestFile)
	if err != nil {
		return err
	}
	listed := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		full, err := safeJoin(root, entry.Path)
		if err != nil {
			return err
		}
		hash, err := SHA256File(full)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", entry.Path, err)
		}
		if hash != entry.Hash {
			return fmt.Errorf("manifest %s: SHA-256 %s, want %s", entry.Path, hash, entry.Hash)
		}
		listed[entry.Path] = struct{}{}
	}

	err = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		absolute, err := filepath.Abs(filePath)
		if err != nil {
			return err
		}
		if absolute == manifestAbs {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("fixture tree contains non-regular file %s", filePath)
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := listed[rel]; !ok {
			return fmt.Errorf("fixture tree contains unlisted file %s", rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// CorpusFormat identifies a supported corpus encoding.
type CorpusFormat string

const (
	TypletJSONL  CorpusFormat = "typlet-jsonl"
	RawLines     CorpusFormat = "raw-lines"
	JSONFormulas CorpusFormat = "json-formulas"
	KaTeXJSONL   CorpusFormat = "katex-jsonl"
)

// KnownDifferenceEntry is an explicit, reviewed exclusion from strict matching.
type KnownDifferenceEntry struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// CorpusSpec declares one loadable corpus and its exact expected counts.
type CorpusSpec struct {
	Name                 string                 `json:"name"`
	Format               CorpusFormat           `json:"format"`
	CorpusFile           string                 `json:"corpusFile"`
	CommentPrefix        string                 `json:"commentPrefix,omitempty"`
	FixtureFile          string                 `json:"fixtureFile,omitempty"`
	ExpectedCount        int                    `json:"expectedCount"`
	ExpectedFixtureCount int                    `json:"expectedFixtureCount,omitempty"`
	ExpectedFrameCount   *int                   `json:"expectedFrameCount,omitempty"`
	SelectionFile        string                 `json:"selectionFile,omitempty"`
	SelectionKey         string                 `json:"selectionKey,omitempty"`
	KnownDifferences     []KnownDifferenceEntry `json:"knownDifferences,omitempty"`
}

// CorpusManifest is the tracked declaration of all importable corpora.
type CorpusManifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	Corpora       []CorpusSpec `json:"corpora"`
}

// LoadCorpusManifest decodes and validates manifests/corpora.json.
func LoadCorpusManifest(filePath string) (*CorpusManifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var manifest CorpusManifest
	if err := decodeJSONObject(data, &manifest, true); err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	return &manifest, nil
}

// Validate checks corpus declarations for ambiguity and missing count data.
func (m *CorpusManifest) Validate() error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported corpus manifest schema %d", m.SchemaVersion)
	}
	if len(m.Corpora) == 0 {
		return errors.New("corpus manifest has no corpora")
	}
	names := make(map[string]struct{}, len(m.Corpora))
	for i, spec := range m.Corpora {
		if spec.Name == "" {
			return fmt.Errorf("corpus %d has no name", i)
		}
		if _, ok := names[spec.Name]; ok {
			return fmt.Errorf("duplicate corpus name %q", spec.Name)
		}
		names[spec.Name] = struct{}{}
		if !safeRelativePath(spec.CorpusFile) {
			return fmt.Errorf("corpus %q has invalid corpusFile %q", spec.Name, spec.CorpusFile)
		}
		if spec.ExpectedCount <= 0 {
			return fmt.Errorf("corpus %q has non-positive expectedCount", spec.Name)
		}
		switch spec.Format {
		case TypletJSONL:
			if !safeRelativePath(spec.FixtureFile) {
				return fmt.Errorf("corpus %q has invalid fixtureFile %q", spec.Name, spec.FixtureFile)
			}
			if spec.ExpectedFixtureCount <= 0 {
				return fmt.Errorf("corpus %q has non-positive expectedFixtureCount", spec.Name)
			}
			if spec.ExpectedFrameCount == nil {
				return fmt.Errorf("corpus %q has no expectedFrameCount", spec.Name)
			}
			if *spec.ExpectedFrameCount < 0 || *spec.ExpectedFrameCount > spec.ExpectedFixtureCount {
				return fmt.Errorf("corpus %q has invalid expectedFrameCount %d", spec.Name, *spec.ExpectedFrameCount)
			}
		case RawLines, JSONFormulas, KaTeXJSONL:
			if spec.FixtureFile != "" || spec.ExpectedFixtureCount != 0 || spec.ExpectedFrameCount != nil {
				return fmt.Errorf("corpus %q has a fixture for non-Typlet format %q", spec.Name, spec.Format)
			}
		default:
			return fmt.Errorf("corpus %q has unknown format %q", spec.Name, spec.Format)
		}
		if spec.CommentPrefix != "" && spec.Format != RawLines {
			return fmt.Errorf("corpus %q sets commentPrefix for format %q", spec.Name, spec.Format)
		}
		if (spec.SelectionFile == "") != (spec.SelectionKey == "") {
			return fmt.Errorf("corpus %q must set both selectionFile and selectionKey or neither", spec.Name)
		}
		if spec.SelectionFile != "" && !safeRelativePath(spec.SelectionFile) {
			return fmt.Errorf("corpus %q has invalid selectionFile %q", spec.Name, spec.SelectionFile)
		}
		known := make(map[string]struct{}, len(spec.KnownDifferences))
		for _, difference := range spec.KnownDifferences {
			if difference.ID == "" || strings.TrimSpace(difference.Reason) == "" {
				return fmt.Errorf("corpus %q has known difference without ID or reason", spec.Name)
			}
			if _, ok := known[difference.ID]; ok {
				return fmt.Errorf("corpus %q has duplicate known difference %q", spec.Name, difference.ID)
			}
			known[difference.ID] = struct{}{}
		}
	}
	return nil
}

// KnownDifferenceFor returns the reviewed reason for id, if any.
func (s CorpusSpec) KnownDifferenceFor(id string) (string, bool) {
	for _, difference := range s.KnownDifferences {
		if difference.ID == id {
			return difference.Reason, true
		}
	}
	return "", false
}

// TypletSelection records the IDs retained from each larger Typlet corpus.
type TypletSelection struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Source        string              `json:"source"`
	Datasets      map[string][]string `json:"datasets"`
}

// LoadTypletSelection decodes and validates the curated Typlet ID list.
func LoadTypletSelection(filePath string) (*TypletSelection, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var selection TypletSelection
	if err := decodeJSONObject(data, &selection, true); err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	if selection.SchemaVersion != 1 || selection.Source == "" || len(selection.Datasets) == 0 {
		return nil, fmt.Errorf("%s: invalid selection header", filePath)
	}
	datasets := make([]string, 0, len(selection.Datasets))
	for dataset := range selection.Datasets {
		datasets = append(datasets, dataset)
	}
	slices.Sort(datasets)
	for _, dataset := range datasets {
		ids := selection.Datasets[dataset]
		if dataset == "" || len(ids) == 0 {
			return nil, fmt.Errorf("%s: empty dataset %q", filePath, dataset)
		}
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			if id == "" {
				return nil, fmt.Errorf("%s: dataset %q contains an empty ID", filePath, dataset)
			}
			if _, ok := seen[id]; ok {
				return nil, fmt.Errorf("%s: dataset %q contains duplicate ID %q", filePath, dataset, id)
			}
			seen[id] = struct{}{}
		}
	}
	return &selection, nil
}

// IDs returns a copy of the selected IDs for a dataset.
func (s TypletSelection) IDs(key string) ([]string, bool) {
	ids, ok := s.Datasets[key]
	if !ok {
		return nil, false
	}
	return slices.Clone(ids), true
}

// KaTeXCase is one normalized parser or error test transcribed from KaTeX.
type KaTeXCase struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Kind     string `json:"kind"`
	Expected string `json:"expected"`
	Position *int   `json:"position,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
	Upstream string `json:"upstream"`
}

// LoadKaTeXCases decodes the normalized KaTeX case list.
func LoadKaTeXCases(filePath string, expected int) ([]KaTeXCase, error) {
	var cases []KaTeXCase
	err := readJSONLines(filePath, func(_ int, data []byte) error {
		var record KaTeXCase
		if err := decodeJSONObject(data, &record, true); err != nil {
			return err
		}
		if record.Kind != "parse" && record.Kind != "error" {
			return fmt.Errorf("case %q has unknown kind %q", record.ID, record.Kind)
		}
		if record.Expected == "" || record.Upstream == "" {
			return fmt.Errorf("case %q is missing expected behavior or upstream location", record.ID)
		}
		if record.Position != nil && *record.Position < 0 {
			return fmt.Errorf("case %q has negative position", record.ID)
		}
		cases = append(cases, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(cases) != expected {
		return nil, fmt.Errorf("%s: decoded %d cases, want %d", filePath, len(cases), expected)
	}
	if err := validateUniqueIDs(filePath, cases, func(record KaTeXCase) string { return record.ID }); err != nil {
		return nil, err
	}
	return cases, nil
}

func safeRelativePath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func safeJoin(root, relative string) (string, error) {
	if !safeRelativePath(relative) {
		return "", fmt.Errorf("unsafe fixture path %q", relative)
	}
	return filepath.Join(root, filepath.FromSlash(relative)), nil
}
