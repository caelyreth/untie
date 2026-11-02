package fixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// GoldenMismatch is the first deterministic difference between golden data and
// actual output.
type GoldenMismatch struct {
	Name     string
	Format   string
	Line     int
	Byte     int
	Expected string
	Actual   string
}

// String reports a stable, bounded diagnostic suitable for test output.
func (m *GoldenMismatch) String() string {
	if m == nil {
		return ""
	}
	location := m.Name
	if m.Line > 0 {
		location += fmt.Sprintf(":%d", m.Line)
	}
	if m.Byte >= 0 {
		location += fmt.Sprintf(" (byte %d)", m.Byte)
	}
	return fmt.Sprintf("%s %s mismatch: expected %q, got %q", location, m.Format, m.Expected, m.Actual)
}

// CanonicalJSON marshals value with stable map ordering, no HTML escaping, and
// one trailing newline. It is intended for reviewed test snapshots only.
func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// NormalizeJSON validates JSON and returns its canonical representation.
func NormalizeJSON(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, err
	}
	return CanonicalJSON(value)
}

// CompareGolden compares text or byte output and reports the first differing
// line. It never writes or updates golden files.
func CompareGolden(name string, got, want []byte) *GoldenMismatch {
	if bytes.Equal(got, want) {
		return nil
	}
	gotLines := splitGoldenLines(got)
	wantLines := splitGoldenLines(want)
	for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
		var gotLine, wantLine string
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if gotLine != wantLine {
			return &GoldenMismatch{
				Name: name, Format: "text", Line: i + 1, Byte: firstByteDifference(got, want),
				Expected: abbreviate(wantLine), Actual: abbreviate(gotLine),
			}
		}
	}
	return &GoldenMismatch{
		Name: name, Format: "text", Byte: firstByteDifference(got, want),
		Expected: abbreviate(string(want)), Actual: abbreviate(string(got)),
	}
}

// CompareJSONGolden validates both inputs and compares canonical JSON.
func CompareJSONGolden(name string, got, want []byte) (*GoldenMismatch, error) {
	gotCanonical, err := NormalizeJSON(got)
	if err != nil {
		return nil, fmt.Errorf("%s actual JSON: %w", name, err)
	}
	wantCanonical, err := NormalizeJSON(want)
	if err != nil {
		return nil, fmt.Errorf("%s golden JSON: %w", name, err)
	}
	if bytes.Equal(gotCanonical, wantCanonical) {
		return nil, nil
	}
	gotLines := strings.Split(strings.TrimSuffix(string(gotCanonical), "\n"), "\n")
	wantLines := strings.Split(strings.TrimSuffix(string(wantCanonical), "\n"), "\n")
	for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
		var gotLine, wantLine string
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if gotLine != wantLine {
			return &GoldenMismatch{
				Name: name, Format: "json", Line: i + 1, Byte: firstByteDifference(gotCanonical, wantCanonical),
				Expected: abbreviate(wantLine), Actual: abbreviate(gotLine),
			}, nil
		}
	}
	return &GoldenMismatch{
		Name: name, Format: "json", Byte: -1,
		Expected: abbreviate(string(wantCanonical)), Actual: abbreviate(string(gotCanonical)),
	}, nil
}

func splitGoldenLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func firstByteDifference(a, b []byte) int {
	limit := min(len(a), len(b))
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return limit
	}
	return -1
}

func abbreviate(value string) string {
	const limit = 240
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
