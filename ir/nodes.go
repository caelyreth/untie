package ir

import "github.com/caelyreth/untie/source"

// Formula is the validated root returned by a frontend lowering pass.
type Formula struct {
	Root Node
}

// Row is a horizontal sequence. It may contain break and alignment markers.
type Row struct {
	Base
	Items []Node
}

// NewRow constructs a row.
func NewRow(span source.Range, items ...Node) *Row {
	return &Row{
		Base:  NewBase(KindRow, span),
		Items: append([]Node(nil), items...),
	}
}

// Glyph is one semantic symbol or grapheme cluster.
type Glyph struct {
	Base
	Text string
}

// NewGlyph constructs a glyph.
func NewGlyph(span source.Range, text string) *Glyph {
	return &Glyph{Base: NewBase(KindGlyph, span), Text: text}
}

// Text is a shaped text-mode run.
type Text struct {
	Base
	Text     string
	Font     FontRole
	Tracking Length
}

// NewText constructs text in the text face.
func NewText(span source.Range, text string) *Text {
	return &Text{Base: NewBase(KindText, span), Text: text, Font: FontText}
}

// Operator is a named or explicit operator.
type Operator struct {
	Base
	Text string
}

// NewOperator constructs an operator.
func NewOperator(span source.Range, text string) *Operator {
	return &Operator{Base: NewBase(KindOperator, span), Text: text}
}

// Space is an explicit or semantic space.
type Space struct {
	Base
	Type   SpaceKind
	Amount Length
	Weak   bool
}

// NewSpace constructs a space node.
func NewSpace(span source.Range, kind SpaceKind, amount Length) *Space {
	return &Space{Base: NewBase(KindSpace, span), Type: kind, Amount: amount}
}

// Fraction is a vertical, horizontal, skewed, or barless fraction.
type Fraction struct {
	Base
	Numerator   Node
	Denominator Node
	Bar         bool
	Style       FractionStyle
	Role        FractionRole // Semantic operation; zero is an ordinary fraction.
	Padding     Length
}

// NewFraction constructs a fraction.
func NewFraction(span source.Range, numerator, denominator Node) *Fraction {
	return &Fraction{
		Base:        NewBase(KindFraction, span),
		Numerator:   numerator,
		Denominator: denominator,
		Bar:         true,
		Style:       FractionVertical,
	}
}

// Radical is a square or nth root.
type Radical struct {
	Base
	Radicand Node
	Index    Node
}

// NewRadical constructs a radical with an optional index.
func NewRadical(span source.Range, radicand, index Node) *Radical {
	return &Radical{Base: NewBase(KindRadical, span), Radicand: radicand, Index: index}
}

// Scripts attaches scripts and limits to a base.
type Scripts struct {
	Base
	BaseItem    Node
	Top         Node
	Bottom      Node
	TopLeft     Node
	BottomLeft  Node
	TopRight    Node
	BottomRight Node
	Role        ScriptRole // Semantic script purpose; zero is an ordinary script.
}

// NewScripts constructs a scripts node.
func NewScripts(span source.Range, base Node) *Scripts {
	return &Scripts{Base: NewBase(KindScripts, span), BaseItem: base}
}

// Fenced is content surrounded by optional delimiters.
type Fenced struct {
	Base
	Open     Node
	Close    Node
	Body     []Node
	Balanced bool
	Sizing   DelimiterSizing
}

// NewFenced constructs a fenced group.
func NewFenced(span source.Range, body ...Node) *Fenced {
	return &Fenced{
		Base: NewBase(KindFenced, span),
		Body: append([]Node(nil), body...),
	}
}

// SizedDelimiter is one standalone delimiter with an explicit target policy. It
// is distinct from an ordinary delimiter atom and from a paired Fenced group.
type SizedDelimiter struct {
	Base
	Text   string
	Sizing DelimiterSizing
}

// NewSizedDelimiter constructs a standalone explicitly sized delimiter.
func NewSizedDelimiter(span source.Range, text string, sizing DelimiterSizing) *SizedDelimiter {
	return &SizedDelimiter{
		Base:   NewBase(KindSizedDelimiter, span),
		Text:   text,
		Sizing: sizing,
	}
}

// Accent places a mark above or below content.
type Accent struct {
	Base
	Body            Node
	Accent          Node
	Position        Position
	Stretchy        bool // Source explicitly requests a stretchable accent.
	NaturalFit      bool // Layout must select an accent fitting the base width.
	Dotless         bool
	ExactFrameWidth bool
}

// NewAccent constructs an accent.
func NewAccent(span source.Range, body, accent Node, position Position) *Accent {
	return &Accent{
		Base:     NewBase(KindAccent, span),
		Body:     body,
		Accent:   accent,
		Position: position,
		Stretchy: true,
	}
}

// Line places a horizontal rule above or below content.
type Line struct {
	Base
	Body      Node
	Position  Position
	Thickness Length
	Gap       Length
}

// NewLine constructs a line decoration.
func NewLine(span source.Range, body Node, position Position) *Line {
	return &Line{Base: NewBase(KindLine, span), Body: body, Position: position}
}

// Cancel overlays cancel strokes on content.
type Cancel struct {
	Base
	Body      Node
	Cross     bool
	Inverted  bool
	Role      CancelRole // Zero is a cancellation strike.
	Thickness Length
}

// NewCancel constructs a cancel decoration.
func NewCancel(span source.Range, body Node) *Cancel {
	return &Cancel{Base: NewBase(KindCancel, span), Body: body}
}

// TableRow is one row of a table. Cells may contain AlignMarker nodes.
type TableRow struct {
	Cells []Node
	Tag   Node
}

// TableLine is a horizontal or vertical rule boundary.
type TableLine struct {
	Position  int
	Count     int
	Dashed    bool
	Thickness Length
}

// Table is a matrix, array, cases group, or aligned rows.
type Table struct {
	Base
	Rows       []TableRow
	Role       TableRole // Semantic grid kind; zero preserves matrix behavior.
	Columns    []ColumnAlign
	RowGap     Length
	ColumnGap  Length
	Horizontal []TableLine
	Vertical   []TableLine
}

// NewTable constructs an empty table.
func NewTable(span source.Range, rows ...TableRow) *Table {
	return &Table{Base: NewBase(KindTable, span), Rows: append([]TableRow(nil), rows...)}
}

// Multiline is a set of rows used for aligned/gathered math without a table's
// column grid.
type Multiline struct {
	Base
	Rows     [][]Node
	Centered bool
}

// NewMultiline constructs an empty multiline node.
func NewMultiline(span source.Range, rows ...[]Node) *Multiline {
	out := make([][]Node, len(rows))
	for i, row := range rows {
		out[i] = append([]Node(nil), row...)
	}
	return &Multiline{Base: NewBase(KindMultiline, span), Rows: out}
}

// Style applies an explicit math style to body.
type Style struct {
	Base
	Body       Node
	SetSize    bool
	NewSize    MathSize
	SetCramped bool
	Cramped    bool
}

// NewStyle constructs an explicit style wrapper.
func NewStyle(span source.Range, body Node) *Style {
	return &Style{Base: NewBase(KindStyle, span), Body: body}
}

// Color applies an explicit or role paint to body.
type Color struct {
	Base
	Body  Node
	Paint Paint
}

// NewColor constructs a color wrapper.
func NewColor(span source.Range, body Node, paint Paint) *Color {
	return &Color{Base: NewBase(KindColor, span), Body: body, Paint: paint}
}

// Box adds spacing and optional decoration around body.
type Box struct {
	Base
	Body        Node
	Padding     [4]Length // top, right, bottom, left
	Fill        Paint
	HasFill     bool
	Border      Paint
	HasBorder   bool
	BorderWidth Length
	Radius      Length
}

// NewBox constructs an undecorated box.
func NewBox(span source.Range, body Node) *Box {
	return &Box{Base: NewBase(KindBox, span), Body: body}
}

// Phantom hides selected extents of body.
type Phantom struct {
	Base
	Body   Node
	Width  bool
	Height bool
	Depth  bool
}

// NewPhantom constructs a phantom with all extents preserved.
func NewPhantom(span source.Range, body Node) *Phantom {
	return &Phantom{Base: NewBase(KindPhantom, span), Body: body, Width: true, Height: true, Depth: true}
}

// ErrorNode draws recoverable source text in the error role.
type ErrorNode struct {
	Base
	Text string
	Code source.Code
}

// NewError constructs an error node.
func NewError(span source.Range, text string, code source.Code) *ErrorNode {
	base := NewBase(KindError, span)
	base.Paint = Paint{Role: PaintError}
	return &ErrorNode{Base: base, Text: text, Code: code}
}

// Placeholder draws a missing or unavailable content box.
type Placeholder struct {
	Base
	Reason string
}

// NewPlaceholder constructs a placeholder.
func NewPlaceholder(span source.Range, reason string) *Placeholder {
	base := NewBase(KindPlaceholder, span)
	base.Paint = Paint{Role: PaintPlaceholder}
	return &Placeholder{Base: base, Reason: reason}
}

// Break marks a legal line break point.
type Break struct {
	Base
	Break   BreakKind
	Penalty int
}

// NewBreak constructs a break marker.
func NewBreak(span source.Range, kind BreakKind, penalty int) *Break {
	return &Break{Base: NewBase(KindBreak, span), Break: kind, Penalty: penalty}
}

// AlignMarker marks a Typst/TeX alignment point.
type AlignMarker struct {
	Base
}

// NewAlignMarker constructs an alignment marker.
func NewAlignMarker(span source.Range) *AlignMarker {
	return &AlignMarker{Base: NewBase(KindAlignMarker, span)}
}
