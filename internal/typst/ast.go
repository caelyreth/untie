package typst

import "github.com/caelyreth/untie/source"

// astKind is intentionally private. Typst syntax nodes must be lowered before
// crossing the package boundary into the dialect-neutral IR.
type astKind uint8

const (
	astInvalid astKind = iota
	astRow
	astGlyph
	astIdentifier
	astField
	astString
	astNumber
	astShorthand
	astAttach
	astFraction
	astRoot
	astDelimited
	astCall
	astCode
	astArray
	astDict
	astUnary
	astLiteral
	astAlign
	astLinebreak
	astEquation
	astError
)

type astNode struct {
	kind astKind
	span source.Range
	// inCode distinguishes a code-mode expression from math-mode content.
	// Typst exposes modules such as sym and math only through code mode.
	inCode bool

	// text is source text for glyphs, identifiers, and fields; it is the
	// decoded value for strings.
	text  string
	open  string
	close string
	raw   string

	openSpan  source.Range
	closeSpan source.Range

	items []*astNode
	args  []astArg
	value value

	base        *astNode
	top         *astNode
	bottom      *astNode
	topLeft     *astNode
	bottomLeft  *astNode
	topRight    *astNode
	bottomRight *astNode
}

type argKind uint8

const (
	argPositional argKind = iota
	argNamed
	argSpread
)

type astArg struct {
	kind argKind
	span source.Range
	name string
	node *astNode

	// empty marks a positional slot with no expression between separators.
	// Matrix-like functions intentionally interpret these as empty cells.
	empty bool

	nameSpan      source.Range
	colonSpan     source.Range
	spreadSpan    source.Range
	separatorSpan source.Range

	// semicolonAfter records a two-dimensional argument separator. It is
	// significant for mat(), but harmless metadata for other calls.
	semicolonAfter bool
}

func newAST(kind astKind, span source.Range) *astNode {
	return &astNode{
		kind:      kind,
		span:      span,
		openSpan:  source.Detached(),
		closeSpan: source.Detached(),
	}
}
