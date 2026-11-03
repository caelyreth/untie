package alt

import (
	"context"
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func buildDescription(
	ctx context.Context,
	formula *ir.Formula,
	limits source.Limits,
) (*display.AltDescription, error) {
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if formula == nil || nilNode(formula.Root) {
		return nil, ErrInvalidRoot
	}
	limits = limits.WithDefaults()
	builder := descriptionBuilder{
		ctx:    ctx,
		limits: limits,
		active: make(map[ir.Node]bool),
	}
	root, err := builder.node(formula.Root, 0)
	if err != nil {
		return nil, err
	}
	out := &display.AltDescription{
		Version: display.AltDescriptionVersion,
		Root:    root,
	}
	if len(formula.Tags) > 0 {
		out.Tags = make([]display.AltTag, len(formula.Tags))
	}
	for i, tag := range formula.Tags {
		builder.count++
		if builder.count > builder.limits.MaxNodes {
			return nil, fmt.Errorf("%w: tag %d metadata count %d",
				ErrLimitExceeded, i, builder.count)
		}
		out.Tags[i] = display.AltTag{
			Manual:     tag.Manual,
			Starred:    tag.Starred,
			Suppressed: tag.Suppressed,
			Line:       tag.Line,
		}
		if tag.Body == nil {
			continue
		}
		body, err := builder.node(tag.Body, 0)
		if err != nil {
			return nil, fmt.Errorf("tag %d: %w", i, err)
		}
		out.Tags[i].Body = &body
	}
	return out, nil
}

type descriptionBuilder struct {
	ctx       context.Context
	limits    source.Limits
	count     int
	textBytes int
	active    map[ir.Node]bool
}

func (b *descriptionBuilder) node(node ir.Node, depth int) (display.AltNode, error) {
	if err := b.ctx.Err(); err != nil {
		return display.AltNode{}, err
	}
	if nilNode(node) {
		b.count++
		if b.count > b.limits.MaxNodes {
			return display.AltNode{}, fmt.Errorf("%w: node limit %d",
				ErrLimitExceeded, b.limits.MaxNodes)
		}
		if depth > b.limits.MaxNesting {
			return display.AltNode{}, fmt.Errorf("%w: nesting limit %d",
				ErrLimitExceeded, b.limits.MaxNesting)
		}
		out := descriptionError("invalid-ir", "missing node")
		b.textBytes += len(out.Text)
		if b.textBytes > b.limits.MaxTextBytes {
			return display.AltNode{}, fmt.Errorf("%w: structured text exceeds %d bytes",
				ErrLimitExceeded, b.limits.MaxTextBytes)
		}
		return out, nil
	}
	if b.active[node] {
		return display.AltNode{}, fmt.Errorf("%w: cycle detected", ErrInvalidRoot)
	}
	b.count++
	if b.count > b.limits.MaxNodes {
		return display.AltNode{}, fmt.Errorf("%w: node limit %d", ErrLimitExceeded, b.limits.MaxNodes)
	}
	if depth > b.limits.MaxNesting {
		return display.AltNode{}, fmt.Errorf("%w: nesting limit %d", ErrLimitExceeded, b.limits.MaxNesting)
	}
	b.active[node] = true
	defer delete(b.active, node)

	out, err := b.nodeValue(node, depth)
	if err != nil {
		return display.AltNode{}, err
	}
	if !transparentDescriptionRow(node) {
		b.textBytes += len(out.Text)
	}
	if b.textBytes > b.limits.MaxTextBytes {
		return display.AltNode{}, fmt.Errorf("%w: structured text exceeds %d bytes",
			ErrLimitExceeded, b.limits.MaxTextBytes)
	}
	return out, nil
}

func transparentDescriptionRow(node ir.Node) bool {
	row, ok := node.(*ir.Row)
	return ok && len(row.Items) == 1
}

func (b *descriptionBuilder) nodeValue(node ir.Node, depth int) (display.AltNode, error) {
	switch n := node.(type) {
	case *ir.Row:
		// One-item rows are transparent grouping syntax. Keep the IR traversal
		// budget intact, but omit the wrapper from the semantic projection so
		// cosmetic braces do not change localized output.
		if len(n.Items) == 1 {
			return b.node(n.Items[0], depth+1)
		}
		out := display.AltNode{Kind: display.AltKindRow}
		if err := b.appendParts(&out, depth, display.AltPartItem, n.Items); err != nil {
			return display.AltNode{}, err
		}
		return out, nil
	case *ir.Glyph:
		return descriptionLeaf(display.AltKindGlyph, n.Text, n.Variant), nil
	case *ir.Text:
		return descriptionLeaf(display.AltKindText, n.Text, n.Variant), nil
	case *ir.Operator:
		return descriptionLeaf(display.AltKindOperator, n.Text, n.Variant), nil
	case *ir.Space:
		out := display.AltNode{
			Kind:  display.AltKindSpace,
			Space: altSpaceKind(n.Type),
		}
		if n.Type == ir.SpaceExplicit {
			out.SpaceWidth = altSpaceWidth(n.Amount)
		}
		addAltFlag(&out, display.AltFlagWeak, n.Weak)
		return out, nil
	case *ir.Fraction:
		out := display.AltNode{
			Kind:          display.AltKindFraction,
			FractionRole:  altFractionRole(n.Role),
			FractionStyle: altFractionStyle(n.Style),
		}
		addAltFlag(&out, display.AltFlagBar, n.Bar)
		if err := b.appendPart(&out, depth, display.AltPartNumerator, n.Numerator); err != nil {
			return display.AltNode{}, err
		}
		if err := b.appendPart(&out, depth, display.AltPartDenominator, n.Denominator); err != nil {
			return display.AltNode{}, err
		}
		return out, nil
	case *ir.Radical:
		out := display.AltNode{Kind: display.AltKindRadical}
		if err := b.appendPart(&out, depth, display.AltPartRadicand, n.Radicand); err != nil {
			return display.AltNode{}, err
		}
		if n.Index != nil {
			if err := b.appendPart(&out, depth, display.AltPartIndex, n.Index); err != nil {
				return display.AltNode{}, err
			}
		}
		return out, nil
	case *ir.Scripts:
		out := display.AltNode{
			Kind:       display.AltKindScripts,
			ScriptRole: altScriptRole(n.Role),
		}
		for _, field := range []struct {
			role display.AltPartRole
			node ir.Node
		}{
			{display.AltPartBase, n.BaseItem},
			{display.AltPartTop, n.Top},
			{display.AltPartBottom, n.Bottom},
			{display.AltPartTopLeft, n.TopLeft},
			{display.AltPartBottomLeft, n.BottomLeft},
			{display.AltPartTopRight, n.TopRight},
			{display.AltPartBottomRight, n.BottomRight},
		} {
			if field.node == nil {
				continue
			}
			if err := b.appendPart(&out, depth, field.role, field.node); err != nil {
				return display.AltNode{}, err
			}
		}
		return out, nil
	case *ir.Fenced:
		out := display.AltNode{
			Kind:   display.AltKindFenced,
			Sizing: altSizingKind(n.Sizing.Kind),
		}
		addAltFlag(&out, display.AltFlagBalanced, n.Balanced)
		if n.Open != nil {
			if err := b.appendPart(&out, depth, display.AltPartOpen, n.Open); err != nil {
				return display.AltNode{}, err
			}
		}
		if err := b.appendParts(&out, depth, display.AltPartContent, n.Body); err != nil {
			return display.AltNode{}, err
		}
		if n.Close != nil {
			if err := b.appendPart(&out, depth, display.AltPartClose, n.Close); err != nil {
				return display.AltNode{}, err
			}
		}
		return out, nil
	case *ir.SizedDelimiter:
		out := descriptionLeaf(display.AltKindSizedDelimiter, n.Text, n.Variant)
		out.Sizing = altSizingKind(n.Sizing.Kind)
		return out, nil
	case *ir.Accent:
		out := display.AltNode{
			Kind:     display.AltKindAccent,
			Position: altPosition(n.Position),
		}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		if err := b.appendPart(&out, depth, display.AltPartAccent, n.Accent); err != nil {
			return display.AltNode{}, err
		}
		addAltFlag(&out, display.AltFlagStretchy, n.Stretchy)
		addAltFlag(&out, display.AltFlagNaturalFit, n.NaturalFit)
		addAltFlag(&out, display.AltFlagDotless, n.Dotless)
		addAltFlag(&out, display.AltFlagExactFrameWidth, n.ExactFrameWidth)
		return out, nil
	case *ir.Line:
		out := display.AltNode{
			Kind:     display.AltKindLine,
			Position: altPosition(n.Position),
		}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		return out, nil
	case *ir.Cancel:
		out := display.AltNode{
			Kind:       display.AltKindCancel,
			CancelRole: altCancelRole(n.Role),
		}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		addAltFlag(&out, display.AltFlagCross, n.Cross)
		addAltFlag(&out, display.AltFlagInverted, n.Inverted)
		return out, nil
	case *ir.Table:
		out := display.AltNode{
			Kind:      display.AltKindTable,
			TableRole: altTableRole(n.Role),
		}
		if len(n.Rows) > 0 {
			out.Rows = make([]display.AltRow, len(n.Rows))
			for i, row := range n.Rows {
				items, err := b.nodes(row.Cells, depth+1)
				if err != nil {
					return display.AltNode{}, err
				}
				out.Rows[i].Items = items
			}
		}
		if len(n.Columns) > 0 {
			out.Columns = make([]display.AltColumn, len(n.Columns))
			for i, align := range n.Columns {
				out.Columns[i].Align = altColumnAlign(align)
			}
		}
		return out, nil
	case *ir.Multiline:
		out := display.AltNode{Kind: display.AltKindMultiline}
		addAltFlag(&out, display.AltFlagCentered, n.Centered)
		if len(n.Rows) > 0 {
			out.Rows = make([]display.AltRow, len(n.Rows))
			for i, row := range n.Rows {
				items, err := b.nodes(row, depth+1)
				if err != nil {
					return display.AltNode{}, err
				}
				out.Rows[i].Items = items
			}
		}
		return out, nil
	case *ir.Style:
		out := display.AltNode{Kind: display.AltKindStyle}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		if n.SetSize {
			out.Size = altMathSize(n.NewSize)
		}
		if n.SetCramped {
			if n.Cramped {
				addAltFlag(&out, display.AltFlagCramped, true)
			} else {
				addAltFlag(&out, display.AltFlagUncramped, true)
			}
		}
		if n.SetFontSize {
			out.FontSize = &display.AltFontSize{Percent: n.FontSize.Float()}
		}
		return out, nil
	case *ir.Overlay:
		out := display.AltNode{
			Kind:             display.AltKindOverlay,
			OverlayAlignment: altOverlayAlignment(n.Alignment),
		}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		return out, nil
	case *ir.Raise:
		out := display.AltNode{
			Kind:           display.AltKindRaise,
			RaiseDirection: altRaiseDirection(n.Amount),
		}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		return out, nil
	case *ir.Color:
		out := display.AltNode{Kind: display.AltKindColor}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		return out, nil
	case *ir.Box:
		out := display.AltNode{Kind: display.AltKindBox}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		addAltFlag(&out, display.AltFlagFill, n.HasFill)
		addAltFlag(&out, display.AltFlagBorder, n.HasBorder)
		return out, nil
	case *ir.Phantom:
		out := display.AltNode{Kind: display.AltKindPhantom}
		if err := b.appendPart(&out, depth, display.AltPartBody, n.Body); err != nil {
			return display.AltNode{}, err
		}
		addAltFlag(&out, display.AltFlagPhantomWidth, n.Width)
		addAltFlag(&out, display.AltFlagPhantomHeight, n.Height)
		addAltFlag(&out, display.AltFlagPhantomDepth, n.Depth)
		return out, nil
	case *ir.ErrorNode:
		return descriptionError(string(n.Code), n.Text), nil
	case *ir.Placeholder:
		return display.AltNode{
			Kind: display.AltKindPlaceholder,
			Text: n.Reason,
		}, nil
	case *ir.Break:
		return display.AltNode{
			Kind:  display.AltKindBreak,
			Break: altBreakKind(n.Break),
		}, nil
	case *ir.AlignMarker:
		return display.AltNode{Kind: display.AltKindAlignMarker}, nil
	default:
		return descriptionError(string(source.CodeUnsupported), nodeKindName(node.Kind())), nil
	}
}

func (b *descriptionBuilder) appendPart(
	out *display.AltNode,
	depth int,
	role display.AltPartRole,
	node ir.Node,
) error {
	child, err := b.node(node, depth+1)
	if err != nil {
		return err
	}
	out.Parts = append(out.Parts, display.AltPart{Role: role, Node: child})
	return nil
}

func (b *descriptionBuilder) appendParts(
	out *display.AltNode,
	depth int,
	role display.AltPartRole,
	nodes []ir.Node,
) error {
	for _, node := range nodes {
		if err := b.appendPart(out, depth, role, node); err != nil {
			return err
		}
	}
	return nil
}

func (b *descriptionBuilder) nodes(nodes []ir.Node, depth int) ([]display.AltNode, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	out := make([]display.AltNode, len(nodes))
	for i, node := range nodes {
		child, err := b.node(node, depth)
		if err != nil {
			return nil, err
		}
		out[i] = child
	}
	return out, nil
}

func descriptionLeaf(kind display.AltNodeKind, text string, variant ir.Variant) display.AltNode {
	return display.AltNode{
		Kind:    kind,
		Text:    text,
		Variant: altVariant(variant),
	}
}

func descriptionError(code, text string) display.AltNode {
	if code == "" {
		code = string(source.CodeInternal)
	}
	return display.AltNode{
		Kind: display.AltKindError,
		Code: code,
		Text: text,
	}
}

func addAltFlag(node *display.AltNode, flag display.AltFlag, enabled bool) {
	if enabled {
		node.Flags = append(node.Flags, flag)
	}
}

func altVariant(variant ir.Variant) display.AltVariant {
	switch variant {
	case ir.VariantNormal:
		return display.AltVariantNormal
	case ir.VariantItalic:
		return display.AltVariantItalic
	case ir.VariantBold:
		return display.AltVariantBold
	case ir.VariantBoldItalic:
		return display.AltVariantBoldItalic
	case ir.VariantDoubleStruck:
		return display.AltVariantDoubleStruck
	case ir.VariantScript:
		return display.AltVariantScript
	case ir.VariantBoldScript:
		return display.AltVariantBoldScript
	case ir.VariantFraktur:
		return display.AltVariantFraktur
	case ir.VariantBoldFraktur:
		return display.AltVariantBoldFraktur
	case ir.VariantSans:
		return display.AltVariantSans
	case ir.VariantBoldSans:
		return display.AltVariantBoldSans
	case ir.VariantMonospace:
		return display.AltVariantMonospace
	default:
		return ""
	}
}

func altSpaceKind(kind ir.SpaceKind) display.AltSpaceKind {
	switch kind {
	case ir.SpaceInterword:
		return display.AltSpaceInterword
	case ir.SpaceIgnored:
		return display.AltSpaceIgnored
	default:
		return display.AltSpaceExplicit
	}
}

func altSpaceWidth(length ir.Length) display.AltSpaceWidth {
	const scale = 1_000_000
	switch {
	case length.IsZero():
		return display.AltSpaceWidthZero
	case length.Unit == ir.LengthMu && length.Value == 3*scale:
		return display.AltSpaceWidthThin
	case length.Unit == ir.LengthMu && length.Value == 4*scale:
		return display.AltSpaceWidthMedium
	case length.Unit == ir.LengthMu && length.Value == 5*scale:
		return display.AltSpaceWidthThick
	case length.Unit == ir.LengthMu && length.Value == 9*scale:
		return display.AltSpaceWidthEn
	case length.Unit == ir.LengthMu && length.Value == 18*scale:
		return display.AltSpaceWidthQuad
	case length.Unit == ir.LengthMu && length.Value == 36*scale:
		return display.AltSpaceWidthDoubleQuad
	case length.Unit == ir.LengthMu && length.Value == -3*scale:
		return display.AltSpaceWidthNegativeThin
	case length.Unit == ir.LengthMu && length.Value == -4*scale:
		return display.AltSpaceWidthNegativeMedium
	case length.Unit == ir.LengthMu && length.Value == -5*scale:
		return display.AltSpaceWidthNegativeThick
	case length.Unit == ir.LengthEm && length.Value == 500_000:
		return display.AltSpaceWidthEn
	case length.Unit == ir.LengthEm && length.Value == scale:
		return display.AltSpaceWidthQuad
	case length.Unit == ir.LengthEm && length.Value == 2*scale:
		return display.AltSpaceWidthDoubleQuad
	default:
		return display.AltSpaceWidthCustom
	}
}

func altFractionRole(role ir.FractionRole) display.AltFractionRole {
	switch role {
	case ir.FractionRoleBinomial:
		return display.AltFractionBinomial
	case ir.FractionRoleStacked:
		return display.AltFractionStacked
	default:
		return display.AltFractionOrdinary
	}
}

func altFractionStyle(style ir.FractionStyle) display.AltFractionStyle {
	switch style {
	case ir.FractionNoBar:
		return display.AltFractionNoBar
	case ir.FractionSkewed:
		return display.AltFractionSkewed
	case ir.FractionHorizontal:
		return display.AltFractionHorizontal
	default:
		return display.AltFractionVertical
	}
}

func altScriptRole(role ir.ScriptRole) display.AltScriptRole {
	switch role {
	case ir.ScriptRoleLimits:
		return display.AltScriptLimits
	case ir.ScriptRoleOverlay:
		return display.AltScriptOverlay
	case ir.ScriptRoleBraceLabel:
		return display.AltScriptBraceLabel
	default:
		return display.AltScriptOrdinary
	}
}

func altCancelRole(role ir.CancelRole) display.AltCancelRole {
	if role == ir.CancelRoleNegation {
		return display.AltCancelNegation
	}
	return display.AltCancelStrike
}

func altTableRole(role ir.TableRole) display.AltTableRole {
	switch role {
	case ir.TableRoleArray:
		return display.AltTableArray
	case ir.TableRoleCases:
		return display.AltTableCases
	case ir.TableRoleVector:
		return display.AltTableVector
	default:
		return display.AltTableMatrix
	}
}

func altPosition(position ir.Position) display.AltPosition {
	if position == ir.PositionBelow {
		return display.AltPositionBelow
	}
	return display.AltPositionAbove
}

func altBreakKind(kind ir.BreakKind) display.AltBreakKind {
	switch kind {
	case ir.BreakBinary:
		return display.AltBreakBinary
	case ir.BreakExplicit:
		return display.AltBreakExplicit
	default:
		return display.AltBreakRelation
	}
}

func altSizingKind(kind ir.DelimiterSizingKind) display.AltSizingKind {
	switch kind {
	case ir.DelimiterSizingRelative:
		return display.AltSizingRelative
	case ir.DelimiterSizingAbsolute:
		return display.AltSizingAbsolute
	default:
		return display.AltSizingContent
	}
}

func altColumnAlign(align ir.ColumnAlign) display.AltColumnAlign {
	switch align {
	case ir.AlignLeft:
		return display.AltColumnLeft
	case ir.AlignRight:
		return display.AltColumnRight
	default:
		return display.AltColumnCenter
	}
}

func altMathSize(size ir.MathSize) display.AltMathSize {
	switch size {
	case ir.SizeDisplay:
		return display.AltSizeDisplay
	case ir.SizeText:
		return display.AltSizeText
	case ir.SizeScript:
		return display.AltSizeScript
	case ir.SizeScriptScript:
		return display.AltSizeScriptScript
	default:
		return display.AltSizeInherit
	}
}

func altOverlayAlignment(alignment ir.OverlayAlignment) display.AltOverlayAlignment {
	switch alignment {
	case ir.OverlayCenter:
		return display.AltOverlayCenter
	case ir.OverlayEnd:
		return display.AltOverlayEnd
	default:
		return display.AltOverlayStart
	}
}

func altRaiseDirection(amount ir.Length) display.AltRaiseDirection {
	switch {
	case amount.Value > 0:
		return display.AltRaiseUp
	case amount.Value < 0:
		return display.AltRaiseDown
	default:
		return display.AltRaiseZero
	}
}
