package daily

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/caelyreth/untie"
)

const (
	// SchemaVersion is the supported daily-corpus schema.
	SchemaVersion = 1

	maxCases     = 1_000
	maxCaseBytes = 2 << 20
)

var stableID = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`)

// Case is one independently reviewed daily-STEM formula.
type Case struct {
	ID              string      `json:"id"`
	Category        string      `json:"category"`
	EquivalentGroup string      `json:"equivalentGroup,omitempty"`
	SourceSpecific  string      `json:"sourceSpecific,omitempty"`
	Dialect         string      `json:"dialect"`
	Mode            string      `json:"mode"`
	Source          string      `json:"source"`
	Limits          *LimitsSpec `json:"limits,omitempty"`
	Style           *StyleSpec  `json:"style,omitempty"`
	Expect          Expectation `json:"expect"`
	Issue           string      `json:"issue,omitempty"`
}

// LimitsSpec overrides parse limits for one case. Omitted or zero fields use
// the public defaults.
type LimitsSpec struct {
	MaxSourceBytes     int   `json:"maxSourceBytes,omitempty"`
	MaxTokens          int   `json:"maxTokens,omitempty"`
	MaxNesting         int   `json:"maxNesting,omitempty"`
	MaxNodes           int   `json:"maxNodes,omitempty"`
	MaxMacroExpansions int   `json:"maxMacroExpansions,omitempty"`
	MaxExpandedTokens  int   `json:"maxExpandedTokens,omitempty"`
	MaxCallDepth       int   `json:"maxCallDepth,omitempty"`
	MaxTableCells      int   `json:"maxTableCells,omitempty"`
	MaxAssemblyRepeats int   `json:"maxAssemblyRepeats,omitempty"`
	MaxTextBytes       int   `json:"maxTextBytes,omitempty"`
	MaxOutputItems     int   `json:"maxOutputItems,omitempty"`
	MaxPathSegments    int   `json:"maxPathSegments,omitempty"`
	MaxDimension       int64 `json:"maxDimension,omitempty"`
}

// StyleSpec overrides host layout style fields for one case.
type StyleSpec struct {
	Size     float32 `json:"size,omitempty"`
	MaxWidth float32 `json:"maxWidth,omitempty"`
}

// Expectation is a closed model for one acceptance case. There are no
// catch-all skip or known-failure statuses.
type Expectation struct {
	Parse               string   `json:"parse"`
	DiagnosticCodes     []string `json:"diagnosticCodes,omitempty"`
	Layout              string   `json:"layout"`
	LayoutErrorContains []string `json:"layoutErrorContains,omitempty"`
	Alt                 string   `json:"alt,omitempty"`
	AltErrorContains    []string `json:"altErrorContains,omitempty"`
	MinLines            int      `json:"minLines,omitempty"`
}

const (
	parseOK         = "ok"
	parseDiagnostic = "diagnostic"

	layoutOK           = "ok"
	layoutError        = "error"
	layoutMissingGlyph = "missing-glyph"

	altOK    = "ok"
	altError = "error"
)

// Manifest declares exact corpus membership and reviewed classifications.
type Manifest struct {
	SchemaVersion    int               `json:"schemaVersion"`
	CaseFile         string            `json:"caseFile"`
	Count            int               `json:"count"`
	Categories       map[string]int    `json:"categories"`
	Dialects         map[string]int    `json:"dialects"`
	Modes            map[string]int    `json:"modes"`
	CaseIDs          []string          `json:"caseIDs"`
	EquivalentGroups int               `json:"equivalentGroups"`
	SourceSpecific   int               `json:"sourceSpecific"`
	ExpectedIssues   map[string]string `json:"expectedIssues,omitempty"`
}

// LoadCases reads and validates a JSON Lines corpus.
func LoadCases(reader io.Reader) ([]Case, error) {
	if reader == nil {
		return nil, errors.New("daily: nil corpus reader")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), maxCaseBytes)
	cases := make([]Case, 0, 128)
	seen := make(map[string]struct{})
	for line := 1; scanner.Scan(); line++ {
		data := scanner.Bytes()
		if len(strings.TrimSpace(string(data))) == 0 {
			return nil, fmt.Errorf("daily: line %d is blank", line)
		}
		var item Case
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&item); err != nil {
			return nil, fmt.Errorf("daily: line %d: %w", line, err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("daily: line %d: %w", line, err)
		}
		if err := item.Validate(); err != nil {
			return nil, fmt.Errorf("daily: line %d: %w", line, err)
		}
		if _, exists := seen[item.ID]; exists {
			return nil, fmt.Errorf("daily: duplicate case ID %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		cases = append(cases, item)
		if len(cases) > maxCases {
			return nil, fmt.Errorf("daily: corpus exceeds %d cases", maxCases)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("daily: read corpus: %w", err)
	}
	if len(cases) == 0 {
		return nil, errors.New("daily: corpus is empty")
	}
	return cases, nil
}

// LoadCaseFile reads a tracked JSON Lines corpus.
func LoadCaseFile(path string) ([]Case, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return LoadCases(file)
}

// LoadManifest reads and strictly decodes a corpus manifest.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("daily: decode manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Manifest{}, fmt.Errorf("daily: manifest: %w", err)
	}
	return manifest, nil
}

// Validate checks one corpus record.
func (c Case) Validate() error {
	if !stableID.MatchString(c.ID) {
		return fmt.Errorf("case %q: ID is not stable", c.ID)
	}
	if !stableID.MatchString(c.Category) {
		return fmt.Errorf("case %q: category %q is not stable", c.ID, c.Category)
	}
	if (c.EquivalentGroup == "") == (c.SourceSpecific == "") {
		return fmt.Errorf("case %q: exactly one of equivalentGroup or sourceSpecific is required", c.ID)
	}
	if c.EquivalentGroup != "" && !stableID.MatchString(c.EquivalentGroup) {
		return fmt.Errorf("case %q: equivalent group %q is not stable", c.ID, c.EquivalentGroup)
	}
	if c.SourceSpecific != "" && !stableID.MatchString(c.SourceSpecific) {
		return fmt.Errorf("case %q: source-specific reason %q is not stable", c.ID, c.SourceSpecific)
	}
	if c.Dialect != "tex" && c.Dialect != "typst" {
		return fmt.Errorf("case %q: unknown dialect %q", c.ID, c.Dialect)
	}
	if c.Mode != "inline" && c.Mode != "display" {
		return fmt.Errorf("case %q: unknown mode %q", c.ID, c.Mode)
	}
	if c.Source == "" {
		return fmt.Errorf("case %q: source is empty", c.ID)
	}
	if c.Issue != "" && !stableID.MatchString(c.Issue) {
		return fmt.Errorf("case %q: issue %q is not stable", c.ID, c.Issue)
	}
	if c.Limits != nil {
		for _, field := range []struct {
			name  string
			value int
		}{
			{"maxSourceBytes", c.Limits.MaxSourceBytes},
			{"maxTokens", c.Limits.MaxTokens},
			{"maxNesting", c.Limits.MaxNesting},
			{"maxNodes", c.Limits.MaxNodes},
			{"maxMacroExpansions", c.Limits.MaxMacroExpansions},
			{"maxExpandedTokens", c.Limits.MaxExpandedTokens},
			{"maxCallDepth", c.Limits.MaxCallDepth},
			{"maxTableCells", c.Limits.MaxTableCells},
			{"maxAssemblyRepeats", c.Limits.MaxAssemblyRepeats},
			{"maxTextBytes", c.Limits.MaxTextBytes},
			{"maxOutputItems", c.Limits.MaxOutputItems},
			{"maxPathSegments", c.Limits.MaxPathSegments},
		} {
			if field.value < 0 {
				return fmt.Errorf("case %q: %s is negative", c.ID, field.name)
			}
		}
		if c.Limits.MaxDimension < 0 {
			return fmt.Errorf("case %q: maxDimension is negative", c.ID)
		}
	}
	if c.Style != nil {
		if c.Style.Size < 0 || c.Style.MaxWidth < 0 {
			return fmt.Errorf("case %q: negative style value", c.ID)
		}
	}
	if err := c.Expect.validate(c.ID, c.Mode); err != nil {
		return err
	}
	if c.Issue != "" && c.Expect.Parse == parseOK && c.Expect.Layout == layoutOK &&
		(c.Expect.Alt == "" || c.Expect.Alt == altOK) {
		return fmt.Errorf("case %q: a tracked issue requires a non-ok expectation", c.ID)
	}
	return nil
}

func (e Expectation) validate(id, mode string) error {
	switch e.Parse {
	case parseOK:
		if len(e.DiagnosticCodes) != 0 {
			return fmt.Errorf("case %q: clean parse has diagnostic codes", id)
		}
	case parseDiagnostic:
		if len(e.DiagnosticCodes) == 0 {
			return fmt.Errorf("case %q: diagnostic parse has no expected codes", id)
		}
		for _, code := range e.DiagnosticCodes {
			if !stableID.MatchString(code) {
				return fmt.Errorf("case %q: diagnostic code %q is not stable", id, code)
			}
		}
	default:
		return fmt.Errorf("case %q: invalid parse expectation %q", id, e.Parse)
	}

	switch e.Layout {
	case layoutOK:
		if len(e.LayoutErrorContains) != 0 {
			return fmt.Errorf("case %q: successful layout has error fragments", id)
		}
	case layoutError:
		if len(e.LayoutErrorContains) == 0 {
			return fmt.Errorf("case %q: error layout has no required fragments", id)
		}
	case layoutMissingGlyph:
		if len(e.LayoutErrorContains) == 0 {
			return fmt.Errorf("case %q: missing-glyph layout has no required fragments", id)
		}
	default:
		return fmt.Errorf("case %q: invalid layout expectation %q", id, e.Layout)
	}

	switch e.Alt {
	case "", altOK:
		if len(e.AltErrorContains) != 0 {
			return fmt.Errorf("case %q: successful alt has error fragments", id)
		}
	case altError:
		if len(e.AltErrorContains) == 0 {
			return fmt.Errorf("case %q: error alt has no required fragments", id)
		}
	default:
		return fmt.Errorf("case %q: invalid alt expectation %q", id, e.Alt)
	}
	if e.MinLines < 0 {
		return fmt.Errorf("case %q: negative minLines", id)
	}
	if e.MinLines > 0 && mode != "display" {
		return fmt.Errorf("case %q: minLines requires display mode", id)
	}
	return nil
}

// ValidateCorpus checks exact manifest membership, categories, pair shape,
// source-specific classification, and expected issues.
func ValidateCorpus(cases []Case, manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("daily: manifest schema %d, want %d", manifest.SchemaVersion, SchemaVersion)
	}
	if manifest.CaseFile == "" || filepath.Base(manifest.CaseFile) != manifest.CaseFile {
		return fmt.Errorf("daily: invalid case file %q", manifest.CaseFile)
	}
	if manifest.Count != len(cases) {
		return fmt.Errorf("daily: manifest count %d, actual %d", manifest.Count, len(cases))
	}
	if len(manifest.Categories) == 0 {
		return errors.New("daily: manifest has no categories")
	}
	if !slices.IsSorted(manifest.CaseIDs) {
		return errors.New("daily: manifest case IDs are not sorted")
	}

	byID := make(map[string]Case, len(cases))
	categoryCounts := make(map[string]int)
	dialectCounts := make(map[string]int)
	modeCounts := make(map[string]int)
	type groupShape struct {
		dialects map[string]int
		category string
		mode     string
	}
	groups := make(map[string]groupShape)
	sourceSpecific := 0
	issues := make(map[string]string)
	for _, item := range cases {
		if err := item.Validate(); err != nil {
			return err
		}
		if _, exists := byID[item.ID]; exists {
			return fmt.Errorf("daily: duplicate case ID %q", item.ID)
		}
		byID[item.ID] = item
		categoryCounts[item.Category]++
		dialectCounts[item.Dialect]++
		modeCounts[item.Mode]++
		if item.EquivalentGroup != "" {
			group := groups[item.EquivalentGroup]
			if group.dialects == nil {
				group.dialects = make(map[string]int)
				group.category = item.Category
				group.mode = item.Mode
			}
			if group.category != item.Category || group.mode != item.Mode {
				return fmt.Errorf("daily: equivalent group %q mixes category or mode", item.EquivalentGroup)
			}
			group.dialects[item.Dialect]++
			groups[item.EquivalentGroup] = group
		} else {
			sourceSpecific++
		}
		if item.Issue != "" {
			if previous, exists := issues[item.ID]; exists && previous != item.Issue {
				return fmt.Errorf("daily: case %q has conflicting issues", item.ID)
			}
			issues[item.ID] = item.Issue
		}
	}
	if len(manifest.CaseIDs) != len(cases) {
		return fmt.Errorf("daily: manifest lists %d IDs, actual %d", len(manifest.CaseIDs), len(cases))
	}
	for i, id := range manifest.CaseIDs {
		if i > 0 && manifest.CaseIDs[i-1] == id {
			return fmt.Errorf("daily: manifest repeats case ID %q", id)
		}
		if _, exists := byID[id]; !exists {
			return fmt.Errorf("daily: manifest case ID %q is absent", id)
		}
	}
	for id := range byID {
		if !slices.Contains(manifest.CaseIDs, id) {
			return fmt.Errorf("daily: case ID %q is absent from manifest", id)
		}
	}
	if !mapsEqual(categoryCounts, manifest.Categories) {
		return fmt.Errorf("daily: category counts %v, manifest %v", categoryCounts, manifest.Categories)
	}
	if !mapsEqual(dialectCounts, manifest.Dialects) {
		return fmt.Errorf("daily: dialect counts %v, manifest %v", dialectCounts, manifest.Dialects)
	}
	if !mapsEqual(modeCounts, manifest.Modes) {
		return fmt.Errorf("daily: mode counts %v, manifest %v", modeCounts, manifest.Modes)
	}
	for group, shape := range groups {
		if len(shape.dialects) != 2 || shape.dialects["tex"] != 1 || shape.dialects["typst"] != 1 {
			return fmt.Errorf("daily: equivalent group %q must contain one TeX and one Typst case", group)
		}
	}
	if manifest.EquivalentGroups != len(groups) {
		return fmt.Errorf("daily: manifest groups %d, actual %d", manifest.EquivalentGroups, len(groups))
	}
	if manifest.SourceSpecific != sourceSpecific {
		return fmt.Errorf("daily: manifest source-specific %d, actual %d", manifest.SourceSpecific, sourceSpecific)
	}
	if !mapsEqual(issues, manifest.ExpectedIssues) {
		return fmt.Errorf("daily: expected issues %v, manifest %v", issues, manifest.ExpectedIssues)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func mapsEqual[V comparable](left, right map[string]V) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func (l *LimitsSpec) untieLimits() untie.Limits {
	if l == nil {
		return untie.Limits{}
	}
	return untie.Limits{
		MaxSourceBytes:     l.MaxSourceBytes,
		MaxTokens:          l.MaxTokens,
		MaxNesting:         l.MaxNesting,
		MaxNodes:           l.MaxNodes,
		MaxMacroExpansions: l.MaxMacroExpansions,
		MaxExpandedTokens:  l.MaxExpandedTokens,
		MaxCallDepth:       l.MaxCallDepth,
		MaxTableCells:      l.MaxTableCells,
		MaxAssemblyRepeats: l.MaxAssemblyRepeats,
		MaxTextBytes:       l.MaxTextBytes,
		MaxOutputItems:     l.MaxOutputItems,
		MaxPathSegments:    l.MaxPathSegments,
		MaxDimension:       l.MaxDimension,
	}
}
