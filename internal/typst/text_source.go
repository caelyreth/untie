package typst

import (
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// textFromAST preserves decoded string spans captured by the lexer. The
// lexer, not the lowerer, owns quote and escape handling; lowerers must never
// infer offsets from delimiters or escape syntax.
func (l *lowerer) textFromAST(n *astNode) ir.Node {
	if n == nil {
		return l.placeholder(source.Detached(), "missing text")
	}
	if len(n.literalSpans) == 0 {
		return l.text(n.span, n.text)
	}
	clusters := make([]ir.TextClusterSource, 0, len(n.literalSpans))
	for _, span := range n.literalSpans {
		clusters = append(clusters, ir.TextClusterSource{
			TextStart: span.textStart,
			TextEnd:   span.textEnd,
			Source:    span.source,
		})
	}
	return l.textWithClusters(n.span, n.text, clusters)
}

// textFromContentSyntax maps a content token whose decoded text is byte-for-byte
// identical to its source. Symbols, shorthands, and evaluated values whose
// source spelling differs intentionally retain the node-level fallback.
func (l *lowerer) textFromContentSyntax(n *astNode) ir.Node {
	if n == nil || n.span.IsDetached() || l.sourceText(n.span) != n.text {
		if n == nil {
			return l.placeholder(source.Detached(), "missing text")
		}
		return l.text(n.span, n.text)
	}
	clusters := make([]ir.TextClusterSource, 0, utf8.RuneCountInString(n.text))
	for start := 0; start < len(n.text); {
		_, size := utf8.DecodeRuneInString(n.text[start:])
		if size <= 0 {
			return l.text(n.span, n.text)
		}
		clusters = append(clusters, ir.TextClusterSource{
			TextStart: start,
			TextEnd:   start + size,
			Source:    source.NewRange(n.span.Start+start, n.span.Start+start+size),
		})
		start += size
	}
	return l.textWithClusters(n.span, n.text, clusters)
}
