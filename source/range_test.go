package source

import "testing"

func TestRange(t *testing.T) {
	t.Parallel()

	r := NewRange(2, 5)
	if !r.IsValid() || r.Len() != 3 || !r.Contains(2) || r.Contains(5) {
		t.Fatalf("unexpected range behavior: %+v", r)
	}
	if got := string(r.Slice("abcdef")); got != "cde" {
		t.Fatalf("Slice() = %q, want %q", got, "cde")
	}
	if got := Merge(NewRange(3, 4), NewRange(1, 2)); got != NewRange(1, 4) {
		t.Fatalf("Merge() = %+v", got)
	}
	if !Detached().IsDetached() || !Detached().IsValid() {
		t.Fatal("detached range metadata is inconsistent")
	}
}

func TestNormalizeDiagnostics(t *testing.T) {
	t.Parallel()

	got := NormalizeDiagnostics([]Diagnostic{
		{Range: NewRange(5, 6), Code: CodeSyntax, Message: "b"},
		{Range: NewRange(1, 2), Code: CodeSyntax, Message: "z"},
		{Range: NewRange(1, 2), Code: CodeUnknownSymbol, Message: "a"},
		{Range: Range{Start: 2, End: 1}, Code: CodeSyntax, Message: "bad"},
	})
	if len(got) != 3 {
		t.Fatalf("len(NormalizeDiagnostics) = %d, want 3", len(got))
	}
	if got[0].Code != CodeSyntax || got[0].Message != "z" || got[1].Code != CodeUnknownSymbol || got[2].Range.Start != 5 {
		t.Fatalf("unexpected ordering: %+v", got)
	}
}
