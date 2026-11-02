package fixture

import (
	"encoding/json"
	"fmt"
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

// FixtureDiagnostic is an opaque upstream diagnostic. Its JSON is retained
// because diagnostics from different Typst layers do not share one shape.
type FixtureDiagnostic struct {
	Raw json.RawMessage
}

// UnmarshalJSON retains the complete diagnostic record.
func (d *FixtureDiagnostic) UnmarshalJSON(data []byte) error {
	d.Raw = append(d.Raw[:0], data...)
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
	Frame       *Frame              `json:"frame"`
	Diagnostics []FixtureDiagnostic `json:"diagnostics,omitempty"`
}

// LoadFormulas decodes a Typlet JSON Lines formula corpus.
func LoadFormulas(path string) ([]Formula, error) {
	var records []Formula
	err := readJSONLines(path, false, func(_ int, data []byte) error {
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
	err := readJSONLines(path, false, func(_ int, data []byte) error {
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
