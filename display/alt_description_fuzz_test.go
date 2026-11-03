package display

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzAltDescriptionOperations(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3})
	f.Add([]byte{255, 254, 253})
	f.Fuzz(func(t *testing.T, data []byte) {
		description := randomAltDescription(data)
		_ = description.Validate()
		clone := description.Clone()
		_ = clone.Validate()
		first, err := description.MarshalCanonical()
		if err != nil {
			return
		}
		second, err := clone.MarshalCanonical()
		if err != nil {
			t.Fatalf("clone canonicalization: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatal("clone changed canonical description")
		}
		var decoded AltDescription
		if err := json.Unmarshal(first, &decoded); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if err := decoded.Validate(); err != nil {
			t.Fatalf("decoded Validate() error = %v", err)
		}
	})
}

func randomAltDescription(data []byte) *AltDescription {
	next := func() byte {
		if len(data) == 0 {
			return 0
		}
		value := data[0]
		data = data[1:]
		return value
	}
	text := func() string {
		n := int(next() % 8)
		out := make([]byte, n)
		for i := range out {
			out[i] = 'a' + next()%26
		}
		return string(out)
	}
	kind := []AltNodeKind{
		AltKindGlyph,
		AltKindText,
		AltKindOperator,
		AltKindError,
		AltKindPlaceholder,
	}[int(next())%5]
	root := AltNode{Kind: kind, Text: text()}
	if kind == AltKindError {
		root.Code = "syntax"
	}
	if next()%2 == 0 {
		root = AltNode{
			Kind: AltKindRow,
			Parts: []AltPart{
				{Role: AltPartItem, Node: root},
			},
		}
	}
	description := &AltDescription{Version: AltDescriptionVersion, Root: root}
	if next()%3 == 0 {
		description.Tags = []AltTag{{
			Manual:  true,
			Starred: next()%2 == 0,
			Line:    FormulaTagLine,
			Body: &AltNode{
				Kind: AltKindText,
				Text: text(),
			},
		}}
	}
	if next()%5 == 0 {
		description.Version = int(next() % 3)
	}
	if next()%7 == 0 {
		description.Root.Kind = ""
	}
	return description
}
