package fixture

import (
	"strings"
	"testing"
)

func TestCanonicalJSONIsStable(t *testing.T) {
	t.Parallel()
	a, err := CanonicalJSON(map[string]any{"b": 2, "a": []any{"<x>", 1}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := CanonicalJSON(map[string]any{"a": []any{"<x>", 1}, "b": 2})
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("canonical JSON differs:\n%s\n%s", a, b)
	}
	if !strings.Contains(string(a), "<x>") || strings.Contains(string(a), `\u003c`) {
		t.Fatalf("unexpected string escaping: %s", a)
	}
}

func TestCompareGoldenReportsFirstDeterministicDifference(t *testing.T) {
	t.Parallel()
	diff := CompareGolden("snapshot", []byte("a\nb\nc\n"), []byte("a\nx\nc\n"))
	if diff == nil || diff.Line != 2 || diff.Expected != "x" || diff.Actual != "b" {
		t.Fatalf("unexpected diff: %#v", diff)
	}
	if first, second := diff.String(), CompareGolden("snapshot", []byte("a\nb\nc\n"), []byte("a\nx\nc\n")).String(); first != second {
		t.Fatalf("nondeterministic diff:\n%s\n%s", first, second)
	}
}

func TestCompareJSONGoldenNormalizesFormatting(t *testing.T) {
	t.Parallel()
	diff, err := CompareJSONGolden("json", []byte(`{"b":2,"a":1}`), []byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if diff != nil {
		t.Fatalf("semantically equal JSON differed: %s", diff)
	}
	if _, err := CompareJSONGolden("invalid", []byte("{"), []byte("{}")); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
