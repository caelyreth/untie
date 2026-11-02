package typst

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
)

type typstFrameRecord struct {
	ID    string `json:"id"`
	Frame *struct {
		Glyphs [][]json.RawMessage `json:"glyphs"`
	} `json:"frame"`
}

type typstSourceRecord struct {
	ID      string `json:"id"`
	Source  string `json:"src"`
	Display bool   `json:"display"`
}

func TestCasesMatchPinnedFixtureBraceSides(t *testing.T) {
	t.Parallel()

	records := loadTypstFrameRecords(t, filepath.Join(
		"..", "..", "testdata", "upstream", "typlet-0.2.0",
		"fixtures", "typst-0.15.1", "paired.jsonl",
	))
	sources := loadTypstSourceRecords(t, filepath.Join(
		"..", "..", "testdata", "upstream", "typlet-0.2.0",
		"corpus", "paired.jsonl",
	))
	for _, test := range []struct {
		id    string
		open  string
		close string
	}{
		{id: "cases", open: "{", close: ""},
		{id: "rcases", open: "", close: "}"},
	} {
		record, ok := records[test.id]
		if !ok {
			t.Fatalf("fixture has no %q record", test.id)
		}
		if record.Frame == nil {
			t.Fatalf("fixture %q has no frame", test.id)
		}
		source, ok := sources[test.id]
		if !ok {
			t.Fatalf("paired corpus has no %q source", test.id)
		}

		var fixtureBraces []string
		for _, glyph := range record.Frame.Glyphs {
			if len(glyph) == 0 {
				continue
			}
			var text string
			if err := json.Unmarshal(glyph[len(glyph)-1], &text); err != nil {
				t.Fatalf("fixture %q glyph: %v", test.id, err)
			}
			if strings.ContainsAny(text, "{}") {
				fixtureBraces = append(fixtureBraces, text)
			}
		}
		if len(fixtureBraces) != 1 || fixtureBraces[0] != test.open+test.close {
			t.Fatalf("fixture %q braces = %v, want one %q", test.id, fixtureBraces, test.open+test.close)
		}

		mode := ir.Inline
		if source.Display {
			mode = ir.Display
		}
		result := Parse(source.Source, mode, Options{})
		requireNoDiagnostics(t, test.id, result)
		fenced := findCasesFence(t, result.Root)
		if nodeText(fenced.Open) != test.open || nodeText(fenced.Close) != test.close {
			t.Fatalf("%s delimiters = %q/%q", test.id, nodeText(fenced.Open), nodeText(fenced.Close))
		}
		if fenced.Balanced {
			t.Fatalf("%s one-sided fence was marked balanced", test.id)
		}
	}
}

func findCasesFence(t *testing.T, root ir.Node) *ir.Fenced {
	t.Helper()
	var found *ir.Fenced
	ir.Walk(root, func(node ir.Node) bool {
		fenced, ok := node.(*ir.Fenced)
		if !ok {
			return true
		}
		if strings.ContainsAny(nodeText(fenced.Open)+nodeText(fenced.Close), "{}") {
			found = fenced
			return false
		}
		return true
	})
	if found == nil {
		t.Fatalf("did not find cases fence in %T", root)
	}
	return found
}

func loadTypstSourceRecords(t *testing.T, path string) map[string]typstSourceRecord {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	records := make(map[string]typstSourceRecord)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var record typstSourceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		records[record.ID] = record
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func loadTypstFrameRecords(t *testing.T, path string) map[string]typstFrameRecord {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	records := make(map[string]typstFrameRecord)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var record typstFrameRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		records[record.ID] = record
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}
