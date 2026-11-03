package tex

import (
	"reflect"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTextClusterSourcesPreserveEscapesAndUnicode(t *testing.T) {
	t.Parallel()

	src := `\text{a\%中}`
	result := Parse(src, ir.Inline, Options{})
	text := requireSingleText(t, result.Root)
	if text.Text != "a%中" {
		t.Fatalf("text = %q, want %q", text.Text, "a%中")
	}
	a := strings.Index(src, "a")
	escaped := strings.Index(src, `\%`)
	unicode := strings.Index(src, "中")
	want := []ir.TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(a, a+1)},
		{TextStart: 1, TextEnd: 2, Source: source.NewRange(escaped, escaped+2)},
		{TextStart: 2, TextEnd: 5, Source: source.NewRange(unicode, unicode+len("中"))},
	}
	if !reflect.DeepEqual(text.Clusters, want) {
		t.Fatalf("clusters = %#v, want %#v", text.Clusters, want)
	}
	if err := ir.ValidateSource(&ir.Formula{Root: result.Root}, src, source.Limits{}); err != nil {
		t.Fatalf("ValidateSource() error = %v", err)
	}
}

func TestTextClusterSourcesUseMacroInvocationOrigin(t *testing.T) {
	t.Parallel()

	src := `\def\foo{A}\text{\foo}`
	result := Parse(src, ir.Inline, Options{})
	text := requireSingleText(t, result.Root)
	invocation := strings.LastIndex(src, `\foo`)
	want := []ir.TextClusterSource{{
		TextStart: 0,
		TextEnd:   1,
		Source:    source.NewRange(invocation, invocation+len(`\foo`)),
	}}
	if !reflect.DeepEqual(text.Clusters, want) {
		t.Fatalf("clusters = %#v, want invocation origin %#v", text.Clusters, want)
	}
}

func TestTextClusterSourcesCoalesceMacroBodyAndArgument(t *testing.T) {
	t.Parallel()

	src := `\def\foo#1{A#1B}\text{\foo{中}}`
	result := Parse(src, ir.Inline, Options{})
	text := requireSingleText(t, result.Root)
	if text.Text != "A中B" {
		t.Fatalf("text = %q, want %q", text.Text, "A中B")
	}
	invocation := strings.LastIndex(src, `\foo`)
	want := []ir.TextClusterSource{{
		TextStart: 0,
		TextEnd:   len(text.Text),
		Source:    source.NewRange(invocation, len(src)-1),
	}}
	if !reflect.DeepEqual(text.Clusters, want) {
		t.Fatalf("clusters = %#v, want coalesced invocation origin %#v", text.Clusters, want)
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
