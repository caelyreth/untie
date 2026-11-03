package mathml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	internalalt "github.com/caelyreth/untie/internal/alt"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// Write emits deterministic Presentation MathML to out.
//
// The formula is validated before any bytes are written. Write may have
// produced a prefix when it later returns a context, writer, or output-limit
// error; callers that require atomic output should use Marshal.
func Write(ctx context.Context, out io.Writer, f *ir.Formula, mode ir.Mode, options Options) error {
	if ctx == nil {
		return ErrInvalidContext
	}
	if out == nil {
		return fmt.Errorf("%w: nil writer", ErrInvalidFormula)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if mode != ir.Inline && mode != ir.Display {
		return fmt.Errorf("%w: unknown mode %d", ErrInvalidFormula, mode)
	}
	options = options.normalized()
	if err := validateExportFormula(f, mode, options.Limits); err != nil {
		return err
	}

	alt, err := internalalt.GenerateFormulaContext(ctx, f, mode, options.Limits)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: generate accessibility text: %v", ErrInvalidFormula, err)
	}

	limited := &contextLimitWriter{
		ctx:   ctx,
		out:   out,
		limit: options.MaxOutputBytes,
	}
	encoder := xml.NewEncoder(limited)
	state := writer{
		ctx:     ctx,
		encoder: encoder,
		mode:    mode,
		limits:  options.Limits,
	}
	if err := state.writeMath(f, accessibilityText{text: alt.Text, speech: alt.Speech}); err != nil {
		return err
	}
	if err := encoder.Flush(); err != nil {
		return writerError(ctx, err)
	}
	return ctx.Err()
}

type contextLimitWriter struct {
	ctx     context.Context
	out     io.Writer
	limit   int
	written int
}

func (w *contextLimitWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > w.limit-w.written {
		return 0, fmt.Errorf("%w: output exceeds %d bytes", ErrLimitExceeded, w.limit)
	}
	n, err := w.out.Write(p)
	w.written += n
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return n, err
	}
	return n, nil
}

type writer struct {
	ctx     context.Context
	encoder *xml.Encoder
	mode    ir.Mode
	limits  source.Limits
	count   int
}

func validateExportFormula(f *ir.Formula, mode ir.Mode, limits source.Limits) error {
	if f == nil {
		return fmt.Errorf("%w: nil formula", ErrInvalidFormula)
	}
	if err := ir.Validate(f, limits); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFormula, err)
	}
	visible, suppressed := 0, 0
	for i, tag := range f.Tags {
		if tag.Line != ir.FormulaTagLine {
			return fmt.Errorf("%w: tag %d uses unsupported line %d", ErrInvalidFormula, i, tag.Line)
		}
		if tag.Suppressed {
			suppressed++
			continue
		}
		visible++
	}
	if visible > 0 && mode != ir.Display {
		return fmt.Errorf("%w: equation tags require display mode", ErrInvalidFormula)
	}
	if visible > 1 {
		return fmt.Errorf("%w: multiple visible formula tags cannot be represented", ErrInvalidFormula)
	}
	_ = suppressed
	return nil
}

func (w *writer) writeMath(f *ir.Formula, alt accessibilityText) error {
	attrs := []xml.Attr{
		attr("xmlns", Namespace),
		attr("display", modeAttribute(w.mode)),
		attr(attrVersion, strconv.Itoa(SchemaVersion)),
	}
	if alt.speech != "" {
		attrs = append(attrs, attr("alttext", alt.speech))
	}
	if alt.text != "" && alt.text != alt.speech {
		attrs = append(attrs, attr(attrAltText, alt.text))
	}
	suppressed := suppressedTagCount(f)
	if suppressed > 0 {
		attrs = append(attrs, attr(attrSuppressed, strconv.Itoa(suppressed)))
	}
	start := xml.StartElement{Name: localName("math"), Attr: attrs}
	if err := w.start(start); err != nil {
		return err
	}

	visibleTag, hasVisibleTag := formulaTag(f)
	if hasVisibleTag {
		tableAttrs := []xml.Attr{dataAttr(attrKind, "formula-tags")}
		if err := w.start(xml.StartElement{Name: localName("mtable"), Attr: tableAttrs}); err != nil {
			return err
		}
		rowAttrs := []xml.Attr{
			dataAttr(attrTagLine, strconv.Itoa(visibleTag.Line)),
			dataAttr(attrTagManual, boolString(visibleTag.Manual)),
			dataAttr(attrTagSuppressed, boolString(visibleTag.Suppressed)),
			dataAttr(attrTagStarred, boolString(visibleTag.Starred)),
		}
		if err := w.start(xml.StartElement{Name: localName("mlabeledtr"), Attr: rowAttrs}); err != nil {
			return err
		}
		if err := w.start(xml.StartElement{Name: localName("mtd")}); err != nil {
			return err
		}
		if err := w.node(visibleTag.Body, 0); err != nil {
			return err
		}
		if err := w.end("mtd"); err != nil {
			return err
		}
		if err := w.start(xml.StartElement{Name: localName("mtd")}); err != nil {
			return err
		}
		if err := w.node(f.Root, 0); err != nil {
			return err
		}
		if err := w.end("mtd"); err != nil {
			return err
		}
		if err := w.end("mlabeledtr"); err != nil {
			return err
		}
		if err := w.end("mtable"); err != nil {
			return err
		}
	} else if err := w.node(f.Root, 0); err != nil {
		return err
	}
	return w.end("math")
}

type accessibilityText struct {
	text   string
	speech string
}

func formulaTag(f *ir.Formula) (ir.EquationTag, bool) {
	for _, tag := range f.Tags {
		if !tag.Suppressed && tag.Body != nil {
			return tag, true
		}
	}
	return ir.EquationTag{}, false
}

func suppressedTagCount(f *ir.Formula) int {
	count := 0
	for _, tag := range f.Tags {
		if tag.Suppressed {
			count++
		}
	}
	return count
}

func modeAttribute(mode ir.Mode) string {
	if mode == ir.Display {
		return "block"
	}
	return "inline"
}

func (w *writer) node(n ir.Node, depth int) error {
	if err := w.check(depth); err != nil {
		return err
	}
	if n == nil {
		return fmt.Errorf("%w: nil node", ErrInvalidFormula)
	}
	base := n.NodeBase()
	attrs, err := baseAttributes(base)
	if err != nil {
		return err
	}

	switch n := n.(type) {
	case *ir.Row:
		return w.container("mrow", attrs, n.Items, depth)
	case *ir.Glyph:
		name := glyphElement(n)
		attrs = appendMOAttrs(attrs, n.Class)
		return w.textElement(name, attrs, n.Text)
	case *ir.Text:
		attrs = append(attrs, dataAttr(attrFont, enumOrInvalid(fontNames[:], n.Font)))
		if !n.Tracking.IsZero() {
			value, err := formatLength(n.Tracking)
			if err != nil {
				return err
			}
			attrs = append(attrs, dataAttr(attrTracking, value))
			attrs = append(attrs, attr("style", "letter-spacing: "+value))
		}
		return w.textElement("mtext", attrs, n.Text)
	case *ir.Operator:
		attrs = appendMOAttrs(attrs, n.Class)
		return w.textElement("mo", attrs, n.Text)
	case *ir.Space:
		width := "0"
		switch n.Type {
		case ir.SpaceExplicit:
			width, err = formatLength(n.Amount)
			if err != nil {
				return err
			}
		case ir.SpaceInterword:
			width = "0.25em"
		case ir.SpaceIgnored:
			width = "0"
		}
		attrs = append(attrs,
			dataAttr(attrSpaceType, enumOrInvalid(spaceTypeNames[:], n.Type)),
			dataAttr(attrSpaceAmount, mustFormatLength(n.Amount)),
			dataAttr(attrSpaceWeak, boolString(n.Weak)),
			attr("width", width),
		)
		return w.empty("mspace", attrs)
	case *ir.Fraction:
		attrs = append(attrs,
			dataAttr(attrFractionStyle, enumOrInvalid(fractionStyleNames[:], n.Style)),
			dataAttr(attrFractionRole, enumOrInvalid(fractionRoleNames[:], n.Role)),
			dataAttr(attrFractionPad, mustFormatLength(n.Padding)),
		)
		if !n.Bar {
			attrs = append(attrs, attr("linethickness", "0"))
		}
		if n.Style == ir.FractionSkewed {
			attrs = append(attrs, attr("bevelled", "true"))
		}
		return w.pair("mfrac", attrs, n.Numerator, n.Denominator, depth)
	case *ir.Radical:
		if n.Index == nil {
			return w.wrap("msqrt", attrs, n.Radicand, depth)
		}
		return w.pair("mroot", attrs, n.Radicand, n.Index, depth)
	case *ir.Scripts:
		return w.scripts(n, attrs, depth)
	case *ir.Fenced:
		return w.fenced(n, attrs, depth)
	case *ir.SizedDelimiter:
		attrs = append(attrs,
			dataAttr(attrSizingKind, enumOrInvalid(sizingKindNames[:], n.Sizing.Kind)),
			dataAttr(attrSizingValue, mustFormatLength(n.Sizing.Value)),
		)
		if n.Sizing.Kind == ir.DelimiterSizingAbsolute && !n.Sizing.Value.IsZero() {
			attrs = append(attrs, attr("minsize", mustFormatLength(n.Sizing.Value)))
		}
		attrs = appendMOAttrs(attrs, n.Class)
		return w.textElement("mo", attrs, n.Text)
	case *ir.Accent:
		name := "mover"
		accentAttr := "accent"
		if n.Position == ir.PositionBelow {
			name = "munder"
			accentAttr = "accentunder"
		}
		attrs = append(attrs,
			attr(accentAttr, "true"),
			dataAttr(attrPosition, enumOrInvalid([]string{"above", "below"}, n.Position)),
			dataAttr(attrStretchy, boolString(n.Stretchy)),
			dataAttr(attrNaturalFit, boolString(n.NaturalFit)),
			dataAttr(attrDotless, boolString(n.Dotless)),
			dataAttr(attrExactWidth, boolString(n.ExactFrameWidth)),
		)
		return w.pair(name, attrs, n.Body, n.Accent, depth)
	case *ir.Line:
		notation := "top"
		if n.Position == ir.PositionBelow {
			notation = "bottom"
		}
		attrs = append(attrs,
			attr("notation", notation),
			dataAttr(attrPosition, enumOrInvalid([]string{"above", "below"}, n.Position)),
			dataAttr(attrThickness, mustFormatLength(n.Thickness)),
			dataAttr(attrGap, mustFormatLength(n.Gap)),
		)
		return w.wrap("menclose", attrs, n.Body, depth)
	case *ir.Cancel:
		notation := "updiagonalstrike"
		if n.Inverted {
			notation = "downdiagonalstrike"
		}
		if n.Cross {
			notation = "updiagonalstrike downdiagonalstrike"
		}
		attrs = append(attrs,
			attr("notation", notation),
			dataAttr(attrCross, boolString(n.Cross)),
			dataAttr(attrInverted, boolString(n.Inverted)),
			dataAttr(attrCancelRole, enumOrInvalid(cancelRoleNames[:], n.Role)),
			dataAttr(attrThickness, mustFormatLength(n.Thickness)),
		)
		return w.wrap("menclose", attrs, n.Body, depth)
	case *ir.Table:
		return w.table(n, attrs, depth)
	case *ir.Multiline:
		attrs = append(attrs,
			dataAttr(attrCentered, boolString(n.Centered)),
		)
		return w.multiline(n, attrs, depth)
	case *ir.Style:
		attrs = append(attrs,
			dataAttr(attrSetSize, boolString(n.SetSize)),
			dataAttr(attrSetCramped, boolString(n.SetCramped)),
			dataAttr(attrStyleSize, enumOrInvalid(sizeNames[:], n.NewSize)),
			dataAttr(attrStyleCramped, boolString(n.Cramped)),
		)
		var styleAttrs []xml.Attr
		styleAttrs = appendSizeAttrs(styleAttrs, n.NewSize, n.SetSize)
		return w.nestedStyle(attrs, styleAttrs, n.Body, depth)
	case *ir.Color:
		return w.nestedStyle(attrs, paintAttributes(nil, n.Paint, "mathcolor"), n.Body, depth)
	case *ir.Box:
		return w.box(n, attrs, depth)
	case *ir.Phantom:
		attrs = append(attrs,
			dataAttr(attrWidthExtent, boolString(n.Width)),
			dataAttr(attrHeightExtent, boolString(n.Height)),
			dataAttr(attrDepthExtent, boolString(n.Depth)),
		)
		return w.wrap("mphantom", attrs, n.Body, depth)
	case *ir.ErrorNode:
		attrs = append(attrs, dataAttr(attrErrorCode, string(n.Code)))
		if err := w.start(xml.StartElement{Name: localName("merror"), Attr: attrs}); err != nil {
			return err
		}
		if err := w.textElement("mtext", nil, n.Text); err != nil {
			return err
		}
		return w.end("merror")
	case *ir.Placeholder:
		attrs = append(attrs, dataAttr(attrReason, n.Reason))
		if err := w.start(xml.StartElement{Name: localName("merror"), Attr: attrs}); err != nil {
			return err
		}
		if err := w.textElement("mtext", nil, "[missing: "+n.Reason+"]"); err != nil {
			return err
		}
		return w.end("merror")
	case *ir.Break:
		attrs = append(attrs,
			dataAttr(attrBreakKind, enumOrInvalid(breakKindNames[:], n.Break)),
			attr("width", "0"),
		)
		return w.empty("mspace", attrs)
	case *ir.AlignMarker:
		attrs = append(attrs, attr("width", "0"))
		return w.empty("mspace", attrs)
	default:
		return fmt.Errorf("%w: unsupported node %T", ErrInvalidFormula, n)
	}
}

func (w *writer) check(depth int) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	w.count++
	if w.count > w.limits.MaxNodes {
		return fmt.Errorf("%w: node budget", ErrLimitExceeded)
	}
	if depth > w.limits.MaxNesting {
		return fmt.Errorf("%w: nesting budget", ErrLimitExceeded)
	}
	return nil
}

func (w *writer) start(start xml.StartElement) error {
	seen := make(map[xml.Name]struct{}, len(start.Attr))
	for _, attr := range start.Attr {
		if _, exists := seen[attr.Name]; exists {
			return fmt.Errorf("%w: duplicate output attribute %q", ErrInvalidFormula, attr.Name.Local)
		}
		seen[attr.Name] = struct{}{}
		if !validXMLText(attr.Value) {
			return fmt.Errorf("%w: attribute %q contains an XML 1.0-invalid character", ErrInvalidFormula, attr.Name.Local)
		}
	}
	return writerError(w.ctx, w.encoder.EncodeToken(start))
}

func (w *writer) end(name string) error {
	return writerError(w.ctx, w.encoder.EncodeToken(xml.EndElement{Name: localName(name)}))
}

func (w *writer) empty(name string, attrs []xml.Attr) error {
	start := xml.StartElement{Name: localName(name), Attr: attrs}
	if err := w.start(start); err != nil {
		return err
	}
	return w.end(name)
}

func (w *writer) textElement(name string, attrs []xml.Attr, text string) error {
	if !validXMLText(text) {
		return fmt.Errorf("%w: %s contains an XML 1.0-invalid character", ErrInvalidFormula, name)
	}
	if err := w.start(xml.StartElement{Name: localName(name), Attr: attrs}); err != nil {
		return err
	}
	if err := writerError(w.ctx, w.encoder.EncodeToken(xml.CharData([]byte(text)))); err != nil {
		return err
	}
	return w.end(name)
}

func (w *writer) wrap(name string, attrs []xml.Attr, child ir.Node, depth int) error {
	if err := w.start(xml.StartElement{Name: localName(name), Attr: attrs}); err != nil {
		return err
	}
	if child != nil {
		if err := w.node(child, depth+1); err != nil {
			return err
		}
	}
	return w.end(name)
}

func (w *writer) nestedStyle(outerAttrs, innerAttrs []xml.Attr, child ir.Node, depth int) error {
	if err := w.start(xml.StartElement{Name: localName("mstyle"), Attr: outerAttrs}); err != nil {
		return err
	}
	if len(innerAttrs) == 0 {
		if err := w.node(child, depth+1); err != nil {
			return err
		}
		return w.end("mstyle")
	}
	innerAttrs = append(innerAttrs, dataAttr(attrStyleWrapper, "true"))
	if err := w.start(xml.StartElement{Name: localName("mstyle"), Attr: innerAttrs}); err != nil {
		return err
	}
	if err := w.node(child, depth+2); err != nil {
		return err
	}
	if err := w.end("mstyle"); err != nil {
		return err
	}
	return w.end("mstyle")
}

func (w *writer) pair(name string, attrs []xml.Attr, first, second ir.Node, depth int) error {
	if err := w.start(xml.StartElement{Name: localName(name), Attr: attrs}); err != nil {
		return err
	}
	if err := w.node(first, depth+1); err != nil {
		return err
	}
	if err := w.node(second, depth+1); err != nil {
		return err
	}
	return w.end(name)
}

func (w *writer) container(name string, attrs []xml.Attr, children []ir.Node, depth int) error {
	if err := w.start(xml.StartElement{Name: localName(name), Attr: attrs}); err != nil {
		return err
	}
	for _, child := range children {
		if err := w.node(child, depth+1); err != nil {
			return err
		}
	}
	return w.end(name)
}

func (w *writer) scripts(n *ir.Scripts, attrs []xml.Attr, depth int) error {
	attrs = append(attrs, dataAttr(attrScriptRole, enumOrInvalid(scriptRoleNames[:], n.Role)))
	hasCorners := n.TopLeft != nil || n.BottomLeft != nil || n.TopRight != nil || n.BottomRight != nil
	if hasCorners {
		attrs = append(attrs, dataAttr(attrScriptLayout, "multiscripts"))
		if n.Top == nil && n.Bottom == nil {
			return w.multiscripts(n, attrs, depth)
		}
		name := scriptsElement(n.Top, n.Bottom, true)
		outerAttrs := []xml.Attr{dataAttr(attrScriptLayout, "multiscripts-limits")}
		if err := w.start(xml.StartElement{Name: localName(name), Attr: outerAttrs}); err != nil {
			return err
		}
		if err := w.multiscripts(n, attrs, depth+1); err != nil {
			return err
		}
		if n.Bottom != nil {
			if err := w.node(n.Bottom, depth+1); err != nil {
				return err
			}
		}
		if n.Top != nil {
			if err := w.node(n.Top, depth+1); err != nil {
				return err
			}
		}
		return w.end(name)
	}

	limits := scriptsUseLimits(n, w.mode)
	layout := "side"
	if limits {
		layout = "limits"
	}
	attrs = append(attrs, dataAttr(attrScriptLayout, layout))
	name := scriptsElement(n.Top, n.Bottom, limits)
	if err := w.start(xml.StartElement{Name: localName(name), Attr: attrs}); err != nil {
		return err
	}
	if err := w.node(n.BaseItem, depth+1); err != nil {
		return err
	}
	if limits {
		if n.Bottom != nil {
			if err := w.node(n.Bottom, depth+1); err != nil {
				return err
			}
		}
		if n.Top != nil {
			if err := w.node(n.Top, depth+1); err != nil {
				return err
			}
		}
	} else {
		if n.Bottom != nil {
			if err := w.node(n.Bottom, depth+1); err != nil {
				return err
			}
		}
		if n.Top != nil {
			if err := w.node(n.Top, depth+1); err != nil {
				return err
			}
		}
	}
	return w.end(name)
}

func (w *writer) multiscripts(n *ir.Scripts, attrs []xml.Attr, depth int) error {
	if err := w.start(xml.StartElement{Name: localName("mmultiscripts"), Attr: attrs}); err != nil {
		return err
	}
	if err := w.node(n.BaseItem, depth+1); err != nil {
		return err
	}
	if err := w.optionalSlot(n.BottomRight, depth+1); err != nil {
		return err
	}
	if err := w.optionalSlot(n.TopRight, depth+1); err != nil {
		return err
	}
	if err := w.empty("mprescripts", nil); err != nil {
		return err
	}
	if err := w.optionalSlot(n.BottomLeft, depth+1); err != nil {
		return err
	}
	if err := w.optionalSlot(n.TopLeft, depth+1); err != nil {
		return err
	}
	if err := w.end("mmultiscripts"); err != nil {
		return err
	}
	return nil
}

func (w *writer) optionalSlot(n ir.Node, depth int) error {
	if n == nil {
		return w.empty("mrow", nil)
	}
	return w.node(n, depth)
}

func (w *writer) fenced(n *ir.Fenced, attrs []xml.Attr, depth int) error {
	attrs = append(attrs,
		dataAttr(attrBalanced, boolString(n.Balanced)),
		dataAttr(attrHasOpen, boolString(n.Open != nil)),
		dataAttr(attrHasClose, boolString(n.Close != nil)),
		dataAttr(attrSizingKind, enumOrInvalid(sizingKindNames[:], n.Sizing.Kind)),
		dataAttr(attrSizingValue, mustFormatLength(n.Sizing.Value)),
	)
	if err := w.start(xml.StartElement{Name: localName("mrow"), Attr: attrs}); err != nil {
		return err
	}
	if n.Open != nil {
		if err := w.node(n.Open, depth+1); err != nil {
			return err
		}
	}
	for _, child := range n.Body {
		if err := w.node(child, depth+1); err != nil {
			return err
		}
	}
	if n.Close != nil {
		if err := w.node(n.Close, depth+1); err != nil {
			return err
		}
	}
	return w.end("mrow")
}

func (w *writer) table(n *ir.Table, attrs []xml.Attr, depth int) error {
	attrs = append(attrs,
		dataAttr(attrTableRole, enumOrInvalid(tableRoleNames[:], n.Role)),
		dataAttr(attrRowGap, mustFormatLength(n.RowGap)),
		dataAttr(attrColumnGap, mustFormatLength(n.ColumnGap)),
		dataAttr(attrHSeparators, formatTableLines(n.Horizontal)),
		dataAttr(attrVSeparators, formatTableLines(n.Vertical)),
	)
	if len(n.Horizontal) > 0 {
		rowLines := "solid"
		for _, line := range n.Horizontal {
			if !line.Dashed {
				rowLines = "solid"
				break
			}
			rowLines = "dashed"
		}
		attrs = append(attrs, attr("rowlines", rowLines))
	}
	if len(n.Vertical) > 0 {
		columnLines := "solid"
		for _, line := range n.Vertical {
			if !line.Dashed {
				columnLines = "solid"
				break
			}
			columnLines = "dashed"
		}
		attrs = append(attrs, attr("columnlines", columnLines))
	}
	if len(n.Columns) > 0 {
		alignments := make([]string, len(n.Columns))
		for i, alignment := range n.Columns {
			alignments[i] = enumOrInvalid([]string{"center", "left", "right"}, alignment)
		}
		attrs = append(attrs, attr("columnalign", strings.Join(alignments, " ")))
	}
	if !n.RowGap.IsZero() {
		attrs = append(attrs, attr("rowspacing", mustFormatLength(n.RowGap)))
	}
	if !n.ColumnGap.IsZero() {
		attrs = append(attrs, attr("columnspacing", mustFormatLength(n.ColumnGap)))
	}
	if err := w.start(xml.StartElement{Name: localName("mtable"), Attr: attrs}); err != nil {
		return err
	}
	for _, row := range n.Rows {
		if err := w.start(xml.StartElement{Name: localName("mtr")}); err != nil {
			return err
		}
		for _, cell := range row.Cells {
			cellAttrs := []xml.Attr{dataAttr(attrKind, "table-cell")}
			if err := w.start(xml.StartElement{Name: localName("mtd"), Attr: cellAttrs}); err != nil {
				return err
			}
			if err := w.node(cell, depth+2); err != nil {
				return err
			}
			if err := w.end("mtd"); err != nil {
				return err
			}
		}
		if err := w.end("mtr"); err != nil {
			return err
		}
	}
	return w.end("mtable")
}

func (w *writer) multiline(n *ir.Multiline, attrs []xml.Attr, depth int) error {
	if err := w.start(xml.StartElement{Name: localName("mtable"), Attr: attrs}); err != nil {
		return err
	}
	for _, row := range n.Rows {
		if err := w.start(xml.StartElement{Name: localName("mtr")}); err != nil {
			return err
		}
		if err := w.start(xml.StartElement{Name: localName("mtd")}); err != nil {
			return err
		}
		for _, child := range row {
			if err := w.node(child, depth+2); err != nil {
				return err
			}
		}
		if err := w.end("mtd"); err != nil {
			return err
		}
		if err := w.end("mtr"); err != nil {
			return err
		}
	}
	return w.end("mtable")
}

func (w *writer) box(n *ir.Box, attrs []xml.Attr, depth int) error {
	attrs = append(attrs,
		dataAttr(attrPaddingTop, mustFormatLength(n.Padding[0])),
		dataAttr(attrPaddingRight, mustFormatLength(n.Padding[1])),
		dataAttr(attrPaddingBottom, mustFormatLength(n.Padding[2])),
		dataAttr(attrPaddingLeft, mustFormatLength(n.Padding[3])),
		dataAttr(attrHasFill, boolString(n.HasFill)),
		dataAttr(attrFill, formatPaint(n.Fill)),
		dataAttr(attrHasBorder, boolString(n.HasBorder)),
		dataAttr(attrBorder, formatPaint(n.Border)),
		dataAttr(attrBorderWidth, mustFormatLength(n.BorderWidth)),
		dataAttr(attrRadius, mustFormatLength(n.Radius)),
	)
	var css []string
	if n.HasFill && n.Fill.HasExplicit {
		attrs = append(attrs, attr("mathbackground", formatRGBA(n.Fill.Explicit)))
	}
	if n.HasBorder && n.Border.HasExplicit {
		css = append(css, "border: "+mustFormatLength(n.BorderWidth)+" solid "+formatRGBA(n.Border.Explicit))
	}
	if !n.Padding[0].IsZero() || !n.Padding[1].IsZero() || !n.Padding[2].IsZero() || !n.Padding[3].IsZero() {
		css = append(css, "padding: "+strings.Join([]string{
			mustFormatLength(n.Padding[0]),
			mustFormatLength(n.Padding[1]),
			mustFormatLength(n.Padding[2]),
			mustFormatLength(n.Padding[3]),
		}, " "))
	}
	if !n.Radius.IsZero() {
		css = append(css, "border-radius: "+mustFormatLength(n.Radius))
	}
	if len(css) > 0 {
		attrs = append(attrs, attr("style", strings.Join(css, "; ")))
	}
	return w.wrap("mstyle", attrs, n.Body, depth)
}

func scriptsElement(top, bottom ir.Node, limits bool) string {
	if limits {
		switch {
		case top != nil && bottom != nil:
			return "munderover"
		case top != nil:
			return "mover"
		case bottom != nil:
			return "munder"
		}
	}
	switch {
	case top != nil && bottom != nil:
		return "msubsup"
	case top != nil:
		return "msup"
	case bottom != nil:
		return "msub"
	default:
		return "mrow"
	}
}

func scriptsUseLimits(n *ir.Scripts, mode ir.Mode) bool {
	placement := n.Limits
	if placement == ir.LimitsAuto && n.BaseItem != nil {
		placement = n.BaseItem.NodeBase().Limits
	}
	switch placement {
	case ir.LimitsAlways:
		return true
	case ir.LimitsNever:
		return false
	case ir.LimitsDisplay:
		return mode == ir.Display
	}
	class := n.Class
	if n.BaseItem != nil && n.BaseItem.NodeBase().Class != ir.ClassNormal {
		class = n.BaseItem.NodeBase().Class
	}
	return mode == ir.Display && class == ir.ClassLarge
}

func glyphElement(n *ir.Glyph) string {
	switch n.Class {
	case ir.ClassAlphabetic:
		return "mi"
	case ir.ClassNormal:
		if isDigitRun(n.Text) {
			return "mn"
		}
		if isOperatorLike(n.Text) {
			return "mo"
		}
		return "mi"
	case ir.ClassBinary, ir.ClassRelation, ir.ClassOpening, ir.ClassClosing,
		ir.ClassFence, ir.ClassPunctuation, ir.ClassLarge, ir.ClassUnary,
		ir.ClassVarying, ir.ClassDiacritic, ir.ClassSpace, ir.ClassSpecial:
		return "mo"
	default:
		return "mi"
	}
}

func isDigitRun(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isOperatorLike(text string) bool {
	switch text {
	case "+", "-", "−", "=", "<", ">", "≤", "≥", "≠", "±", "∓",
		",", ";", ":", "(", ")", "[", "]", "{", "}", "|", "‖",
		"∑", "∏", "∫", "∬", "∭", "∮":
		return true
	default:
		return false
	}
}

func appendMOAttrs(attrs []xml.Attr, class ir.MathClass) []xml.Attr {
	switch class {
	case ir.ClassOpening:
		return append(attrs, attr("fence", "true"), attr("form", "prefix"))
	case ir.ClassClosing:
		return append(attrs, attr("fence", "true"), attr("form", "postfix"))
	case ir.ClassFence:
		return append(attrs, attr("fence", "true"))
	case ir.ClassPunctuation:
		return append(attrs, attr("separator", "true"))
	case ir.ClassBinary, ir.ClassRelation:
		return append(attrs, attr("form", "infix"))
	case ir.ClassLarge:
		return append(attrs, attr("largeop", "true"))
	case ir.ClassUnary:
		return append(attrs, attr("form", "prefix"))
	default:
		return attrs
	}
}

func baseAttributes(base *ir.Base) ([]xml.Attr, error) {
	if base == nil {
		return nil, fmt.Errorf("%w: nil base", ErrInvalidFormula)
	}
	attrs := []xml.Attr{dataAttr(attrKind, ir.KindName(base.Kind()))}
	if base.NodeID.Assigned() {
		attrs = append(attrs, dataAttr(attrNodeID, strconv.FormatUint(uint64(base.NodeID), 10)))
	}
	attrs = append(attrs, dataAttr(attrClass, enumOrInvalid(classNames[:], base.Class)))
	if base.Size != ir.SizeInherit {
		attrs = append(attrs,
			dataAttr(attrBaseSize, enumOrInvalid(sizeNames[:], base.Size)),
		)
		attrs = appendSizeAttrs(attrs, base.Size, true)
	}
	if base.Cramped {
		attrs = append(attrs, dataAttr(attrBaseCramped, "true"))
	}
	if base.Variant != ir.VariantAuto {
		name, ok := variantNames[base.Variant]
		if !ok {
			return nil, fmt.Errorf("%w: invalid variant %d", ErrInvalidFormula, base.Variant)
		}
		attrs = append(attrs, attr("mathvariant", name))
	}
	attrs = append(attrs, paintAttributes(nil, base.Paint, "mathcolor")...)
	if base.Limits != ir.LimitsAuto {
		attrs = append(attrs, dataAttr(attrLimits, enumOrInvalid(limitsNames[:], base.Limits)))
	}
	if !base.LeftSpacing.IsZero() {
		value, err := formatLength(base.LeftSpacing)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr("lspace", value))
	}
	if !base.RightSpacing.IsZero() {
		value, err := formatLength(base.RightSpacing)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr("rspace", value))
	}
	if base.IgnoreAsSpace {
		attrs = append(attrs, dataAttr(attrIgnoreSpace, "true"))
	}
	return attrs, nil
}

func appendSizeAttrs(attrs []xml.Attr, size ir.MathSize, set bool) []xml.Attr {
	if !set {
		return attrs
	}
	switch size {
	case ir.SizeDisplay:
		return append(attrs, attr("displaystyle", "true"), attr("scriptlevel", "0"))
	case ir.SizeText:
		return append(attrs, attr("displaystyle", "false"))
	case ir.SizeScript:
		return append(attrs, attr("scriptlevel", "1"))
	case ir.SizeScriptScript:
		return append(attrs, attr("scriptlevel", "2"))
	default:
		return attrs
	}
}

func paintAttributes(attrs []xml.Attr, paint ir.Paint, colorAttr string) []xml.Attr {
	if paint.HasExplicit {
		attrs = append(attrs, attr(colorAttr, formatRGBA(paint.Explicit)))
	} else if paint.Role != ir.PaintInk {
		attrs = append(attrs, dataAttr(attrPaintRole, enumOrInvalid(paintRoleNames[:], paint.Role)))
	}
	return attrs
}

func formatPaint(paint ir.Paint) string {
	if paint.HasExplicit {
		return formatRGBA(paint.Explicit)
	}
	return enumOrInvalid(paintRoleNames[:], paint.Role)
}

func formatRGBA(color ir.RGBA) string {
	if color.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", color.R, color.G, color.B, color.A)
}

func formatTableLines(lines []ir.TableLine) string {
	if len(lines) == 0 {
		return ""
	}
	parts := make([]string, len(lines))
	for i, line := range lines {
		parts[i] = strings.Join([]string{
			strconv.Itoa(line.Position),
			strconv.Itoa(line.Count),
			boolString(line.Dashed),
			mustFormatLength(line.Thickness),
		}, ",")
	}
	return strings.Join(parts, ";")
}

func mustFormatLength(length ir.Length) string {
	value, err := formatLength(length)
	if err != nil {
		return ""
	}
	return value
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func attr(name, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Local: name}, Value: value}
}

func dataAttr(name, value string) xml.Attr {
	return attr(name, value)
}

func localName(name string) xml.Name {
	return xml.Name{Local: name}
}

func writerError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, ErrLimitExceeded) {
		return err
	}
	return fmt.Errorf("mathml: write: %w", err)
}
