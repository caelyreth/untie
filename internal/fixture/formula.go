package fixture

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/caelyreth/untie/source"
)

// Formula is one record from a Typlet formula corpus.
type Formula struct {
	ID       string          `json:"id"`
	Source   string          `json:"src"`
	Display  bool            `json:"display"`
	Preamble string          `json:"preamble,omitempty"`
	TeX      string          `json:"tex,omitempty"`
	Category string          `json:"cat,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// FixtureDiagnostic is an upstream diagnostic with its stable fields decoded.
// Raw is retained because diagnostics from different Typst layers do not share
// one complete shape.
type FixtureDiagnostic struct {
	Raw      json.RawMessage
	At       string
	Message  string
	Severity string
	Hints    []string
	Span     *UTF16Range
}

// UnmarshalJSON decodes the stable diagnostic fields and retains the complete
// record. A span, when present, is a UTF-16 half-open range.
func (d *FixtureDiagnostic) UnmarshalJSON(data []byte) error {
	var wire struct {
		At       string      `json:"at"`
		Message  string      `json:"message"`
		Severity string      `json:"severity"`
		Hints    []string    `json:"hints"`
		Span     *UTF16Range `json:"span"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("diagnostic: %w", err)
	}
	*d = FixtureDiagnostic{
		Raw: cloneRaw(data), At: wire.At, Message: wire.Message,
		Severity: wire.Severity, Hints: append([]string(nil), wire.Hints...),
	}
	if wire.Span != nil {
		span := *wire.Span
		d.Span = &span
	}
	return nil
}

// MarshalJSON restores the complete diagnostic record.
func (d FixtureDiagnostic) MarshalJSON() ([]byte, error) {
	if len(d.Raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), d.Raw...), nil
}

// Fixture is one Typlet fixture record. Semantic payloads other than ID,
// diagnostics, and the flattened frame are intentionally not decoded.
type Fixture struct {
	ID          string              `json:"id"`
	CST         *CSTNode            `json:"cst,omitempty"`
	Frame       *Frame              `json:"frame"`
	Diagnostics []FixtureDiagnostic `json:"diagnostics,omitempty"`
}

// ResolvedDiagnostic is a diagnostic whose optional span has been converted to
// a UTF-8 byte range. At identifies whether Span addresses Src or Preamble.
type ResolvedDiagnostic struct {
	Raw      json.RawMessage
	At       string
	Message  string
	Severity string
	Hints    []string
	HasSpan  bool
	Span     source.Range
}

// ResolvedFixture contains the fixture payloads that use source offsets.
type ResolvedFixture struct {
	CST         *SourceCSTNode
	Diagnostics []ResolvedDiagnostic
}

// ResolveFixture converts all CST and diagnostic UTF-16 offsets to UTF-8 byte
// ranges. Diagnostics with At set to "src" use src; "preamble" diagnostics use
// preamble.
func ResolveFixture(src, preamble string, fixture *Fixture) (*ResolvedFixture, error) {
	if fixture == nil {
		return nil, errors.New("nil fixture")
	}
	resolved := &ResolvedFixture{}
	if fixture.CST != nil {
		cst, err := ResolveCST(src, fixture.CST)
		if err != nil {
			return nil, fmt.Errorf("fixture %q CST: %w", fixture.ID, err)
		}
		resolved.CST = cst
	}
	resolved.Diagnostics = make([]ResolvedDiagnostic, len(fixture.Diagnostics))
	for i, diagnostic := range fixture.Diagnostics {
		if err := diagnostic.Validate(); err != nil {
			return nil, fmt.Errorf("fixture %q diagnostic %d: %w", fixture.ID, i, err)
		}
		item := ResolvedDiagnostic{
			Raw: cloneRaw(diagnostic.Raw), At: diagnostic.At,
			Message: diagnostic.Message, Severity: diagnostic.Severity,
			Hints: append([]string(nil), diagnostic.Hints...),
		}
		if diagnostic.Span != nil {
			var target string
			switch diagnostic.At {
			case "src":
				target = src
			case "preamble":
				target = preamble
			default:
				return nil, fmt.Errorf("fixture %q diagnostic %d has span for unknown target %q",
					fixture.ID, i, diagnostic.At)
			}
			span, err := ConvertUTF16Range(target, *diagnostic.Span)
			if err != nil {
				return nil, fmt.Errorf("fixture %q diagnostic %d: %w", fixture.ID, i, err)
			}
			item.HasSpan = true
			item.Span = span
		}
		resolved.Diagnostics[i] = item
	}
	return resolved, nil
}

// Validate checks stable diagnostic fields without a source string.
func (d FixtureDiagnostic) Validate() error {
	if d.Span != nil {
		if err := d.Span.Validate(); err != nil {
			return err
		}
		if d.At == "" {
			return errors.New("diagnostic span has no target")
		}
	}
	return nil
}

// LoadFormulas decodes a Typlet JSON Lines formula corpus.
func LoadFormulas(path string) ([]Formula, error) {
	var records []Formula
	err := readJSONLines(path, func(_ int, data []byte) error {
		var record Formula
		if err := decodeJSONObject(data, &record, false); err != nil {
			return err
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := validateUniqueIDs(path, records, func(record Formula) string { return record.ID }); err != nil {
		return nil, err
	}
	return records, nil
}

// LoadFixtures decodes a Typlet JSON Lines fixture file.
func LoadFixtures(path string) ([]Fixture, error) {
	var records []Fixture
	err := readJSONLines(path, func(_ int, data []byte) error {
		var record Fixture
		if err := decodeJSONObject(data, &record, false); err != nil {
			return err
		}
		if record.ID == "" {
			return fmt.Errorf("empty fixture ID")
		}
		if record.Frame != nil {
			if err := record.Frame.Validate(); err != nil {
				return fmt.Errorf("fixture %q: %w", record.ID, err)
			}
		}
		if record.CST != nil {
			if err := record.CST.Validate(); err != nil {
				return fmt.Errorf("fixture %q CST: %w", record.ID, err)
			}
		}
		for i := range record.Diagnostics {
			if err := record.Diagnostics[i].Validate(); err != nil {
				return fmt.Errorf("fixture %q diagnostic %d: %w", record.ID, i, err)
			}
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := validateUniqueIDs(path, records, func(record Fixture) string { return record.ID }); err != nil {
		return nil, err
	}
	return records, nil
}
