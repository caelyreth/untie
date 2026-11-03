package typst

import (
	"reflect"
	"strings"
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
	want := []ir.TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(5, 6)},
		{TextStart: 1, TextEnd: 4, Source: source.NewRange(6, 9)},
	}
	if texts[0].Text != "a中" || !reflect.DeepEqual(texts[0].Clusters, want) {
		t.Fatalf("first text = %#v", texts[0])
	}
}

func TestTextContentBlockLowering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "adjacent identifiers", src: `text[a b]`, want: "a b"},
		{name: "comment between identifiers", src: `text[a/*comment*/b]`, want: "ab"},
		{name: "symbol code", src: `text[#sym.alpha]`, want: "α"},
		{name: "qualified symbol code", src: `text[#std.sym.alpha]`, want: "α"},
		{name: "literal code", src: `text[#1]`, want: "1"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := Parse(test.src, ir.Inline, Options{})
			var text strings.Builder
			for _, node := range requireTextNodes(t, result.Root) {
				text.WriteString(node.Text)
				if string(node.Span.Slice(test.src)) == node.Text && strings.TrimSpace(node.Text) != "" &&
					len(node.Clusters) == 0 {
					t.Fatalf("source-identical text %q has no clusters", node.Text)
				}
				assertTextClusters(t, test.src, node)
			}
			if got := text.String(); got != test.want {
				t.Fatalf("text = %q, want %q", got, test.want)
			}
			if err := ir.ValidateSource(&ir.Formula{Root: result.Root}, test.src, source.Limits{}); err != nil {
				t.Fatalf("ValidateSource() error = %v", err)
			}
		})
	}
}

func assertTextClusters(t *testing.T, src string, text *ir.Text) {
	t.Helper()
	covered := make([]int, len(text.Text))
	sourceIdentical := string(text.Span.Slice(src)) == text.Text
	for _, cluster := range text.Clusters {
		if cluster.TextStart < 0 || cluster.TextEnd <= cluster.TextStart || cluster.TextEnd > len(text.Text) {
			t.Fatalf("invalid cluster text range %#v for %q", cluster, text.Text)
		}
		mapped := cluster.Source.Slice(src)
		if mapped == nil || sourceIdentical && string(mapped) != text.Text[cluster.TextStart:cluster.TextEnd] {
			t.Fatalf("cluster %#v does not map %q to the same source", cluster, text.Text[cluster.TextStart:cluster.TextEnd])
		}
		for i := cluster.TextStart; i < cluster.TextEnd; i++ {
			covered[i]++
		}
	}
	for i, count := range covered {
		if len(text.Clusters) > 0 && count != 1 {
			t.Fatalf("text byte %d in %q covered %d times", i, text.Text, count)
		}
	}
}

func requireTextNodes(t *testing.T, root ir.Node) []*ir.Text {
	t.Helper()
	var found []*ir.Text
	ir.Walk(root, func(node ir.Node) bool {
		if text, ok := node.(*ir.Text); ok {
			found = append(found, text)
		}
		return true
	})
	if len(found) == 0 {
		t.Fatal("no text node found")
	}
	return found
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
