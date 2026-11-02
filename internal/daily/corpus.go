package daily

import (
	"bufio"
	"crypto/sha256"
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
	SchemaVersion = 2

	maxCases     = 1_000
	maxCaseBytes = 2 << 20
)

var stableID = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`)

// Case is one independently reviewed daily-STEM formula.
type Case struct {
	ID                 string      `json:"id"`
	Category           string      `json:"category"`
	CoverageGroup      string      `json:"coverageGroup,omitempty"`
	ExpectedDifference string      `json:"expectedDifference,omitempty"`
	SourceSpecific     string      `json:"sourceSpecific,omitempty"`
	Dialect            string      `json:"dialect"`
	Mode               string      `json:"mode"`
	Source             string      `json:"source"`
	Limits             *LimitsSpec `json:"limits,omitempty"`
	Style              *StyleSpec  `json:"style,omitempty"`
	Expect             Expectation `json:"expect"`
	Issue              string      `json:"issue,omitempty"`
}

// LimitsSpec overrides parse and layout limits for one case. Omitted or zero
// fields use the public defaults.
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
	Align    string  `json:"align,omitempty"`
}

// Expectation is a closed model for one acceptance case. There are no
// catch-all skip or known-failure statuses.
type Expectation struct {
	Parse               string           `json:"parse"`
	DiagnosticCodes     []string         `json:"diagnosticCodes,omitempty"`
	Layout              string           `json:"layout"`
	LayoutErrorContains []string         `json:"layoutErrorContains,omitempty"`
	Alt                 string           `json:"alt,omitempty"`
	AltText             string           `json:"altText,omitempty"`
	AltSpeech           string           `json:"altSpeech,omitempty"`
	AltErrorContains    []string         `json:"altErrorContains,omitempty"`
	RootKind            string           `json:"rootKind,omitempty"`
	MinLines            int              `json:"minLines,omitempty"`
	RequireOverflow     bool             `json:"requireOverflow,omitempty"`
	RequireFits         bool             `json:"requireFits,omitempty"`
	RequireTag          bool             `json:"requireTag,omitempty"`
	Hits                []HitExpectation `json:"hits,omitempty"`
}

// HitExpectation pins one source range and semantic hit kind. Text must occur in
// the case source; Occurrence is zero-based.
type HitExpectation struct {
	Text       string `json:"text"`
	Occurrence int    `json:"occurrence,omitempty"`
	Kind       string `json:"kind"`
}

const (
	parseOK         = "ok"
	parseDiagnostic = "diagnostic"

	layoutOK           = "ok"
	layoutError        = "error"
	layoutMissingGlyph = "missing-glyph"

	altOK    = "ok"
	altError = "error"

	alignStart  = "start"
	alignCenter = "center"
	alignEnd    = "end"
)

// sourceSpecificReasons is the closed taxonomy for cases that deliberately
// cover one dialect rather than a paired product scenario.
var sourceSpecificReasons = map[string]struct{}{
	"tex-delimiter-sizing":         {},
	"typst-delimiter-sizing":       {},
	"tex-environment":              {},
	"typst-multiline":              {},
	"tex-recovery":                 {},
	"typst-recovery":               {},
	"tex-resource-limit":           {},
	"typst-resource-limit":         {},
	"display-alignment":            {},
	"display-breaks":               {},
	"display-overflow":             {},
	"tex-equation-tag":             {},
	"tex-equation-tag-alignment":   {},
	"tex-equation-tag-constrained": {},
	"host-font-boundary":           {},
}

// expectedDifferences is the closed taxonomy for why a coverage group is not
// asserting semantic equality between its TeX and Typst cases.
var expectedDifferences = map[string]struct{}{
	"source-syntax":     {},
	"relation-spacing":  {},
	"text-preservation": {},
	"symbol-selection":  {},
	"layout-geometry":   {},
	"alternative-text":  {},
}

var hitKinds = map[string]struct{}{
	"atom": {}, "group": {}, "script": {},
	"fraction-numerator": {}, "fraction-denominator": {}, "radical": {},
	"fence": {}, "table-cell": {}, "row": {}, "other": {},
}

var rootKinds = map[string]struct{}{
	"row": {}, "placeholder": {}, "error": {},
}

// Manifest declares exact corpus membership and reviewed classifications.
type Manifest struct {
	SchemaVersion  int               `json:"schemaVersion"`
	CaseFile       string            `json:"caseFile"`
	ContentSHA256  string            `json:"contentSHA256"`
	Count          int               `json:"count"`
	Categories     map[string]int    `json:"categories"`
	Dialects       map[string]int    `json:"dialects"`
	Modes          map[string]int    `json:"modes"`
	CaseIDs        []string          `json:"caseIDs"`
	CoverageGroups int               `json:"coverageGroups"`
	SourceSpecific int               `json:"sourceSpecific"`
	ExpectedIssues map[string]string `json:"expectedIssues,omitempty"`
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
	if (c.CoverageGroup == "") == (c.SourceSpecific == "") {
		return fmt.Errorf("case %q: exactly one of coverageGroup or sourceSpecific is required", c.ID)
	}
	if c.CoverageGroup != "" && !stableID.MatchString(c.CoverageGroup) {
		return fmt.Errorf("case %q: coverage group %q is not stable", c.ID, c.CoverageGroup)
	}
	if c.CoverageGroup != "" {
		if _, ok := expectedDifferences[c.ExpectedDifference]; !ok {
			return fmt.Errorf("case %q: unknown expected difference %q", c.ID, c.ExpectedDifference)
		}
	} else if c.ExpectedDifference != "" {
		return fmt.Errorf("case %q: source-specific case has an expected difference", c.ID)
	}
	if c.SourceSpecific != "" {
		if _, ok := sourceSpecificReasons[c.SourceSpecific]; !ok {
			return fmt.Errorf("case %q: source-specific reason %q is outside the taxonomy", c.ID, c.SourceSpecific)
		}
		if strings.HasPrefix(c.SourceSpecific, "tex-") && c.Dialect != "tex" {
			return fmt.Errorf("case %q: source-specific reason %q requires the TeX dialect", c.ID, c.SourceSpecific)
		}
		if strings.HasPrefix(c.SourceSpecific, "typst-") && c.Dialect != "typst" {
			return fmt.Errorf("case %q: source-specific reason %q requires the Typst dialect", c.ID, c.SourceSpecific)
		}
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
		switch c.Style.Align {
		case "", alignStart, alignCenter, alignEnd:
		default:
			return fmt.Errorf("case %q: unknown alignment %q", c.ID, c.Style.Align)
		}
		if c.Style.Align != "" && c.Style.MaxWidth <= 0 {
			return fmt.Errorf("case %q: alignment requires a positive maxWidth", c.ID)
		}
		if c.Mode == "inline" && c.Style.MaxWidth > 0 {
			return fmt.Errorf("case %q: inline layout ignores maxWidth", c.ID)
		}
	}
	if (c.Expect.MinLines > 0 || c.Expect.RequireOverflow) &&
		(c.Style == nil || c.Style.MaxWidth <= 0) {
		return fmt.Errorf("case %q: constrained-line expectations require a positive maxWidth", c.ID)
	}
	if c.Expect.RequireFits && (c.Style == nil || c.Style.MaxWidth <= 0) {
		return fmt.Errorf("case %q: requireFits requires a positive maxWidth", c.ID)
	}
	if c.Expect.RequireFits && c.Expect.RequireOverflow {
		return fmt.Errorf("case %q: requireFits conflicts with requireOverflow", c.ID)
	}
	if c.Expect.RequireTag && c.Mode != "display" {
		return fmt.Errorf("case %q: requireTag requires display mode", c.ID)
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
	if (e.AltText == "") != (e.AltSpeech == "") {
		return fmt.Errorf("case %q: exact alt expectations require both text and speech", id)
	}
	if e.AltText != "" && e.Alt == altError {
		return fmt.Errorf("case %q: exact alt expectations cannot be combined with an alt error", id)
	}
	if e.RootKind != "" {
		if _, ok := rootKinds[e.RootKind]; !ok {
			return fmt.Errorf("case %q: unknown root kind %q", id, e.RootKind)
		}
	}
	if e.MinLines < 0 {
		return fmt.Errorf("case %q: negative minLines", id)
	}
	if e.MinLines > 0 && mode != "display" {
		return fmt.Errorf("case %q: minLines requires display mode", id)
	}
	if e.RequireOverflow && mode != "display" {
		return fmt.Errorf("case %q: requireOverflow requires display mode", id)
	}
	if e.RequireFits && mode != "display" {
		return fmt.Errorf("case %q: requireFits requires display mode", id)
	}
	for i, hit := range e.Hits {
		if hit.Text == "" {
			return fmt.Errorf("case %q: hit %d has empty text", id, i)
		}
		if hit.Occurrence < 0 {
			return fmt.Errorf("case %q: hit %d has negative occurrence", id, i)
		}
		if _, ok := hitKinds[hit.Kind]; !ok {
			return fmt.Errorf("case %q: hit %d has unknown kind %q", id, i, hit.Kind)
		}
	}
	return nil
}

// BuildManifest derives every reviewed manifest invariant and content digest
// from cases. It is also the explicit update path used after corpus review.
func BuildManifest(cases []Case, caseFile string) (Manifest, error) {
	if caseFile == "" || filepath.Base(caseFile) != caseFile {
		return Manifest{}, fmt.Errorf("daily: invalid case file %q", caseFile)
	}
	if len(cases) == 0 {
		return Manifest{}, errors.New("daily: corpus is empty")
	}

	byID := make(map[string]Case, len(cases))
	categoryCounts := make(map[string]int)
	dialectCounts := make(map[string]int)
	modeCounts := make(map[string]int)
	type groupShape struct {
		dialects   map[string]int
		category   string
		mode       string
		difference string
	}
	groups := make(map[string]groupShape)
	sourceSpecific := 0
	issues := make(map[string]string)
	for _, item := range cases {
		if err := item.Validate(); err != nil {
			return Manifest{}, err
		}
		if _, exists := byID[item.ID]; exists {
			return Manifest{}, fmt.Errorf("daily: duplicate case ID %q", item.ID)
		}
		byID[item.ID] = item
		categoryCounts[item.Category]++
		dialectCounts[item.Dialect]++
		modeCounts[item.Mode]++
		if item.CoverageGroup != "" {
			group := groups[item.CoverageGroup]
			if group.dialects == nil {
				group.dialects = make(map[string]int)
				group.category = item.Category
				group.mode = item.Mode
				group.difference = item.ExpectedDifference
			}
			if group.category != item.Category || group.mode != item.Mode ||
				group.difference != item.ExpectedDifference {
				return Manifest{}, fmt.Errorf(
					"daily: coverage group %q mixes category, mode, or expected difference",
					item.CoverageGroup,
				)
			}
			group.dialects[item.Dialect]++
			groups[item.CoverageGroup] = group
		} else {
			sourceSpecific++
		}
		if item.Issue != "" {
			issues[item.ID] = item.Issue
		}
	}
	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	slices.Sort(groupNames)
	for _, group := range groupNames {
		shape := groups[group]
		if len(shape.dialects) != 2 || shape.dialects["tex"] != 1 || shape.dialects["typst"] != 1 {
			return Manifest{}, fmt.Errorf("daily: coverage group %q must contain one TeX and one Typst case", group)
		}
	}

	ids := make([]string, len(cases))
	for i, item := range cases {
		ids[i] = item.ID
	}
	slices.Sort(ids)
	manifest := Manifest{
		SchemaVersion:  SchemaVersion,
		CaseFile:       caseFile,
		Count:          len(cases),
		Categories:     categoryCounts,
		Dialects:       dialectCounts,
		Modes:          modeCounts,
		CaseIDs:        ids,
		CoverageGroups: len(groups),
		SourceSpecific: sourceSpecific,
	}
	if len(issues) != 0 {
		manifest.ExpectedIssues = issues
	}
	digest, err := contentDigest(cases, manifest)
	if err != nil {
		return Manifest{}, err
	}
	manifest.ContentSHA256 = digest
	return manifest, nil
}

// ValidateCorpus checks exact manifest invariants and the canonical content
// digest. Any unreviewed case or manifest edit fails closed.
func ValidateCorpus(cases []Case, manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("daily: manifest schema %d, want %d", manifest.SchemaVersion, SchemaVersion)
	}
	actual, err := BuildManifest(cases, manifest.CaseFile)
	if err != nil {
		return err
	}
	if !manifestInvariantsEqual(manifest, actual) {
		return errors.New("daily: manifest invariants are stale; run dailyreport -update-manifest after review")
	}
	if len(manifest.ContentSHA256) != sha256.Size*2 {
		return errors.New("daily: manifest content digest is missing or malformed")
	}
	if manifest.ContentSHA256 != actual.ContentSHA256 {
		return fmt.Errorf(
			"daily: content digest %s, want %s; run dailyreport -update-manifest after review",
			manifest.ContentSHA256,
			actual.ContentSHA256,
		)
	}
	return nil
}

func contentDigest(cases []Case, manifest Manifest) (string, error) {
	ordered := slices.Clone(cases)
	slices.SortFunc(ordered, func(left, right Case) int {
		return strings.Compare(left.ID, right.ID)
	})
	digest := sha256.New()
	_, _ = digest.Write([]byte("untie-daily-stem-v2\n"))
	for _, item := range ordered {
		data, err := json.Marshal(item)
		if err != nil {
			return "", fmt.Errorf("daily: canonical case %q: %w", item.ID, err)
		}
		_, _ = digest.Write(data)
		_, _ = digest.Write([]byte{'\n'})
	}
	invariants := manifest
	invariants.ContentSHA256 = ""
	data, err := json.Marshal(invariants)
	if err != nil {
		return "", fmt.Errorf("daily: canonical manifest: %w", err)
	}
	_, _ = digest.Write(data)
	return fmt.Sprintf("%x", digest.Sum(nil)), nil
}

func manifestInvariantsEqual(left, right Manifest) bool {
	return left.SchemaVersion == right.SchemaVersion &&
		left.CaseFile == right.CaseFile &&
		left.Count == right.Count &&
		mapsEqual(left.Categories, right.Categories) &&
		mapsEqual(left.Dialects, right.Dialects) &&
		mapsEqual(left.Modes, right.Modes) &&
		slices.Equal(left.CaseIDs, right.CaseIDs) &&
		left.CoverageGroups == right.CoverageGroups &&
		left.SourceSpecific == right.SourceSpecific &&
		mapsEqual(left.ExpectedIssues, right.ExpectedIssues)
}

// WriteManifest writes a reviewed manifest update atomically.
func WriteManifest(path string, manifest Manifest) error {
	if manifest.ContentSHA256 == "" {
		return errors.New("daily: refusing to write a manifest without a content digest")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
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
