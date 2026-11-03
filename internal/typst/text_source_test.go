package typst

import (
	"reflect"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestQuotedStringClusterSourcesPreserveEscapesAndUnicode(t *testing.T) {
	t.Parallel()

	src := `text("a\"中")`
	result := Parse(src, ir.Inline, Options{})
	text := requireSingleText(t, result.Root)
	if text.Text != "a\"中" {
		t.Fatalf("text = %q, want %q", text.Text, "a\"中")
	}
	want := []ir.TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(6, 7)},
		{TextStart: 1, TextEnd: 2, Source: source.NewRange(7, 9)},
		{TextStart: 2, TextEnd: 5, Source: source.NewRange(9, 12)},
	}
	if !reflect.DeepEqual(text.Clusters, want) {
		t.Fatalf("clusters = %#v, want %#v", text.Clusters, want)
	}
	if err := ir.ValidateSource(&ir.Formula{Root: result.Root}, src, source.Limits{}); err != nil {
		t.Fatalf("ValidateSource() error = %v", err)
	}
}

func TestQuotedStringUnicodeEscapeMapsWholeEscape(t *testing.T) {
	t.Parallel()

	src := `text("\u{4E2D}")`
	result := Parse(src, ir.Inline, Options{})
	text := requireSingleText(t, result.Root)
	want := []ir.TextClusterSource{{
		TextStart: 0,
		TextEnd:   len("中"),
		Source:    source.NewRange(6, len(src)-2),
	}}
	if !reflect.DeepEqual(text.Clusters, want) {
		t.Fatalf("clusters = %#v, want %#v", text.Clusters, want)
	}
}

func TestTextContentBlockUsesTokenSpan(t *testing.T) {
	t.Parallel()

	src := `text[a中]`
	result := Parse(src, ir.Inline, Options{})
	var texts []*ir.Text
	ir.Walk(result.Root, func(node ir.Node) bool {
		if text, ok := node.(*ir.Text); ok {
			texts = append(texts, text)
		}
		return true
	})
	if len(texts) != 1 {
		t.Fatalf("text node count = %d, want 1", len(texts))
	}
	if texts[0].Text != "a中" || texts[0].Clusters[0].Source != (source.Range{Start: 5, End: 9}) {
		t.Fatalf("first text = %#v", texts[0])
	}
}

func requireSingleText(t *testing.T, root ir.Node) *ir.Text {
	t.Helper()
	var found *ir.Text
	ir.Walk(root, func(node ir.Node) bool {
		text, ok := node.(*ir.Text)
		if !ok {
			return true
		}
		if found != nil {
			t.Fatalf("found multiple text nodes: %q and %q", found.Text, text.Text)
		}
		found = text
		return true
	})
	if found == nil {
		t.Fatal("no text node found")
	}
	return found
}
