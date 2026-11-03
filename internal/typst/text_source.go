package typst

import (
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
