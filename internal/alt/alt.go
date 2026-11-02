package alt

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var (
	// ErrInvalidContext reports a nil context.
	ErrInvalidContext = errors.New("alt: nil context")
	// ErrInvalidRoot reports a nil or malformed expression root.
	ErrInvalidRoot = errors.New("alt: invalid expression root")
	// ErrLimitExceeded reports a traversal or output-size limit.
	ErrLimitExceeded = errors.New("alt: limit exceeded")
	// ErrUnsupportedNode reports a node representation this pass cannot explain.
	ErrUnsupportedNode = errors.New("alt: unsupported node")
)

// Generate derives alternative text with a background context.
func Generate(root ir.Node, mode ir.Mode, limits source.Limits) display.Alt {
	out, _ := GenerateContext(context.Background(), root, mode, limits)
	return out
}

// GenerateContext derives alternative text with cancellation and caller-owned
// traversal/output limits. It returns a visible fallback alongside non-context
// errors so callers that ignore the error still never receive invented text.
func GenerateContext(ctx context.Context, root ir.Node, mode ir.Mode, limits source.Limits) (out display.Alt, err error) {
	effectiveLimits := limits.WithDefaults()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: alternative-text panic", ErrInvalidRoot)
			out = failedAlt(err, effectiveLimits.MaxTextBytes)
		}
	}()
	if ctx == nil {
		return display.Alt{}, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return display.Alt{}, err
	}
	if nilNode(root) {
		return failedAlt(ErrInvalidRoot, effectiveLimits.MaxTextBytes), ErrInvalidRoot
	}
	if mode != ir.Inline && mode != ir.Display {
		return failedAlt(fmt.Errorf("%w: unknown math mode %d", ErrInvalidRoot, mode), effectiveLimits.MaxTextBytes),
			fmt.Errorf("%w: unknown math mode %d", ErrInvalidRoot, mode)
	}

	r := renderer{
		ctx:    ctx,
		limits: effectiveLimits,
		active: make(map[ir.Node]bool),
	}
	parts, err := r.render(root, renderEnv{display: mode == ir.Display}, 0)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return display.Alt{}, ctxErr
		}
		return failedAlt(err, r.limits.MaxTextBytes), err
	}
	out = display.Alt{
		Speech: strings.TrimSpace(parts.speech),
		Text:   strings.TrimSpace(parts.text),
	}
	if err := out.Validate(); err != nil {
		return failedAlt(err, r.limits.MaxTextBytes), fmt.Errorf("%w: %v", ErrInvalidRoot, err)
	}
	return out, r.firstErr
}

type renderer struct {
	ctx      context.Context
	limits   source.Limits
	active   map[ir.Node]bool
	count    int
	firstErr error
}

type renderEnv struct {
	display bool
}

type piece struct {
	text   string
	speech string
	atomic bool
	attach bool
	word   bool
}

func (r *renderer) render(node ir.Node, env renderEnv, depth int) (piece, error) {
	if err := r.ctx.Err(); err != nil {
		return piece{}, err
	}
	if nilNode(node) {
		err := fmt.Errorf("%w: missing node", ErrInvalidRoot)
		r.note(err)
		return marker("missing node"), nil
	}
	if r.active[node] {
		return piece{}, fmt.Errorf("%w: cycle detected", ErrInvalidRoot)
	}
	r.count++
	if r.count > r.limits.MaxNodes {
		return piece{}, fmt.Errorf("%w: node limit %d", ErrLimitExceeded, r.limits.MaxNodes)
	}
	if depth > r.limits.MaxNesting {
		return piece{}, fmt.Errorf("%w: nesting limit %d", ErrLimitExceeded, r.limits.MaxNesting)
	}
	r.active[node] = true
	defer delete(r.active, node)

	var (
		out piece
		err error
	)
	switch n := node.(type) {
	case *ir.Row:
		out, err = r.row(n, env, depth)
	case *ir.Glyph:
		out = r.leaf(n.Text, glyphSpeech(n.Text), n.Variant)
	case *ir.Text:
		out = r.leaf(n.Text, textSpeech(n.Text), n.Variant)
		out.attach = true
		out.word = strings.TrimSpace(n.Text) != ""
	case *ir.Operator:
		out = r.leaf(n.Text, operatorSpeech(n.Text), n.Variant)
		out.word = isWordText(n.Text)
	case *ir.Space:
		out = spacePiece(n)
	case *ir.Fraction:
		out, err = r.fraction(n, env, depth)
	case *ir.Radical:
		out, err = r.radical(n, env, depth)
	case *ir.Scripts:
		out, err = r.scripts(n, env, depth)
	case *ir.Fenced:
		out, err = r.fenced(n, env, depth)
	case *ir.Accent:
		out, err = r.accent(n, env, depth)
	case *ir.Line:
		out, err = r.line(n, env, depth)
	case *ir.Cancel:
		out, err = r.cancel(n, env, depth)
	case *ir.Table:
		out, err = r.table(n, env, depth)
	case *ir.Multiline:
		out, err = r.multiline(n, env, depth)
	case *ir.Style:
		out, err = r.style(n, env, depth)
	case *ir.Color:
		out, err = r.child(n.Body, env, depth)
	case *ir.Box:
		out, err = r.box(n, env, depth)
	case *ir.Phantom:
		out, err = r.phantom(n, env, depth)
	case *ir.ErrorNode:
		out = piece{
			text:   "[error: " + n.Text + "]",
			speech: "error: " + safeSpeech(n.Text),
			atomic: true,
		}
	case *ir.Placeholder:
		out = piece{
			text:   "[missing: " + n.Reason + "]",
			speech: "missing content: " + safeSpeech(n.Reason),
			atomic: true,
		}
	case *ir.Break:
		out = piece{text: ` \\ `, speech: "line break", atomic: true}
	case *ir.AlignMarker:
		out = piece{text: " & ", speech: "alignment point", atomic: true}
	default:
		err := fmt.Errorf("%w: %s", ErrUnsupportedNode, nodeKindName(node.Kind()))
		r.note(err)
		out = marker("unsupported node: " + nodeKindName(node.Kind()))
	}
	if err != nil {
		return piece{}, err
	}
	return r.bounded(out)
}

func (r *renderer) row(n *ir.Row, env renderEnv, depth int) (piece, error) {
	var text strings.Builder
	var speech strings.Builder
	haveSpeech := false
	lastSpace := false
	lastWord := false
	lastText := ""
	atomic := false
	rendered := 0
	for _, child := range n.Items {
		part, err := r.render(child, env, depth+1)
		if err != nil {
			return piece{}, err
		}
		if text.Len()+len(part.text) > r.limits.MaxTextBytes {
			return piece{}, fmt.Errorf("%w: text output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
		}
		if needsTextBoundary(lastText, lastWord, part) {
			if text.Len()+1 > r.limits.MaxTextBytes {
				return piece{}, fmt.Errorf("%w: text output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
			}
			text.WriteByte(' ')
		}
		text.WriteString(part.text)
		if err := r.appendSpeech(&speech, &haveSpeech, &lastSpace, part); err != nil {
			return piece{}, err
		}
		trimmed := strings.TrimSpace(part.text)
		if trimmed != "" {
			rendered++
			atomic = part.atomic
			lastWord = part.word
		} else if part.text != "" {
			lastWord = false
		}
		if part.text != "" {
			lastText = part.text
		}
	}
	return piece{
		text:   text.String(),
		speech: speech.String(),
		atomic: rendered == 1 && atomic,
		word:   rendered == 1 && lastWord,
	}, nil
}

func (r *renderer) fraction(n *ir.Fraction, env renderEnv, depth int) (piece, error) {
	numerator, err := r.render(n.Numerator, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	denominator, err := r.render(n.Denominator, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	switch n.Role {
	case ir.FractionRoleBinomial:
		return piece{
			text:   "binom(" + numerator.text + ", " + denominator.text + ")",
			speech: "binomial coefficient " + numerator.speech + " choose " + denominator.speech,
			atomic: true,
		}, nil
	case ir.FractionRoleStacked:
		return piece{
			text:   "stack(" + numerator.text + ", " + denominator.text + ")",
			speech: numerator.speech + " over " + denominator.speech,
			atomic: true,
		}, nil
	}
	if n.Style == ir.FractionNoBar {
		return piece{
			text:   "stack(" + numerator.text + ", " + denominator.text + ")",
			speech: numerator.speech + " over " + denominator.speech,
			atomic: true,
		}, nil
	}
	verb := " over "
	if n.Style == ir.FractionHorizontal {
		verb = " divided by "
	}
	return piece{
		text:   groupedText(numerator) + "/" + groupedText(denominator),
		speech: numerator.speech + verb + denominator.speech,
		atomic: true,
	}, nil
}

func (r *renderer) radical(n *ir.Radical, env renderEnv, depth int) (piece, error) {
	radicand, err := r.render(n.Radicand, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	var index piece
	if n.Index != nil {
		index, err = r.render(n.Index, env, depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	indexText := strings.TrimSpace(index.text)
	text := "√" + groupedText(radicand)
	speech := "square root of " + radicand.speech
	switch indexText {
	case "", "2":
	case "3":
		text = "∛" + groupedText(radicand)
		speech = "cube root of " + radicand.speech
	case "4":
		text = "∜" + groupedText(radicand)
		speech = "fourth root of " + radicand.speech
	default:
		text = "root(" + index.text + ", " + radicand.text + ")"
		speech = "root of order " + index.speech + " of " + radicand.speech
	}
	return piece{text: text, speech: speech, atomic: true}, nil
}

func (r *renderer) scripts(n *ir.Scripts, env renderEnv, depth int) (piece, error) {
	base, err := r.render(n.BaseItem, scriptEnv(n.Base, env), depth+1)
	if err != nil {
		return piece{}, err
	}
	role := n.Role

	top := piece{}
	if n.Top != nil {
		top, err = r.render(n.Top, scriptEnv(n.Base, env), depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	bottom := piece{}
	if n.Bottom != nil {
		bottom, err = r.render(n.Bottom, scriptEnv(n.Base, env), depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	topRight := piece{}
	if n.TopRight != nil {
		topRight, err = r.render(n.TopRight, scriptEnv(n.Base, env), depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	bottomRight := piece{}
	if n.BottomRight != nil {
		bottomRight, err = r.render(n.BottomRight, scriptEnv(n.Base, env), depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	topLeft := piece{}
	if n.TopLeft != nil {
		topLeft, err = r.render(n.TopLeft, scriptEnv(n.Base, env), depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	bottomLeft := piece{}
	if n.BottomLeft != nil {
		bottomLeft, err = r.render(n.BottomLeft, scriptEnv(n.Base, env), depth+1)
		if err != nil {
			return piece{}, err
		}
	}

	if needsPositionedAttachment(role, top, bottom, topLeft, bottomLeft, topRight, bottomRight) {
		return positionedAttachmentPiece(role, base, top, bottom, topLeft, bottomLeft, topRight, bottomRight), nil
	}

	if role == ir.ScriptRoleOrdinary {
		topRight = appendScriptPiece(top, topRight)
		bottomRight = appendScriptPiece(bottom, bottomRight)
		top = piece{}
		bottom = piece{}
	}

	switch role {
	case ir.ScriptRoleLimits:
		return limitsPiece(base, top, bottom, topLeft, bottomLeft, topRight, bottomRight), nil
	case ir.ScriptRoleOverlay:
		return overlayPiece(base, top, bottom), nil
	case ir.ScriptRoleBraceLabel:
		return braceLabelPiece(base, top, bottom), nil
	default:
		return ordinaryScriptPiece(base, topLeft, bottomLeft, topRight, bottomRight), nil
	}
}

func limitsPiece(base, top, bottom, topLeft, bottomLeft, topRight, bottomRight piece) piece {
	var text strings.Builder
	writeSideScripts(&text, base, topLeft, bottomLeft, bottom, top, topRight, bottomRight)
	speech := base.speech
	if trimmedPieceText(bottom) != "" {
		speech += " from " + bottom.speech
	}
	if trimmedPieceText(top) != "" {
		speech += " to " + top.speech
	}
	speech = appendSideScriptSpeech(speech, topLeft, bottomLeft, topRight, bottomRight)
	return piece{text: text.String(), speech: speech, atomic: true, word: base.word}
}

func ordinaryScriptPiece(base, topLeft, bottomLeft, topRight, bottomRight piece) piece {
	var text strings.Builder
	writeSideScripts(&text, base, topLeft, bottomLeft, piece{}, piece{}, topRight, bottomRight)
	speech := base.speech
	if trimmedPieceText(bottomRight) != "" {
		speech += " sub " + bottomRight.speech
	}
	if trimmedPieceText(topRight) != "" {
		speech += powerSpeech(topRight)
	}
	speech = appendSideScriptSpeech(speech, topLeft, bottomLeft, piece{}, piece{})
	return piece{text: text.String(), speech: speech, atomic: true, word: base.word}
}

func overlayPiece(base, top, bottom piece) piece {
	topText := trimmedPieceText(top)
	bottomText := trimmedPieceText(bottom)
	switch {
	case topText != "" && bottomText != "":
		return piece{
			text:   "overlay(" + top.text + ", " + base.text + ", " + bottom.text + ")",
			speech: base.speech + " with " + top.speech + " above and " + bottom.speech + " below",
			atomic: true,
		}
	case topText != "":
		return piece{
			text:   "overset(" + top.text + ", " + base.text + ")",
			speech: base.speech + " with " + top.speech + " above",
			atomic: true,
		}
	case bottomText != "":
		return piece{
			text:   "underset(" + bottom.text + ", " + base.text + ")",
			speech: base.speech + " with " + bottom.speech + " below",
			atomic: true,
		}
	default:
		return base
	}
}

func braceLabelPiece(base, top, bottom piece) piece {
	if trimmedPieceText(top) != "" && trimmedPieceText(bottom) != "" {
		return positionedAttachmentPiece(ir.ScriptRoleBraceLabel, base, top, bottom, piece{}, piece{}, piece{}, piece{})
	}
	label := top
	if trimmedPieceText(label) == "" {
		label = bottom
	}
	if trimmedPieceText(label) == "" {
		return base
	}
	return piece{
		text:   "labeled(" + base.text + ", " + label.text + ")",
		speech: base.speech + " labeled " + label.speech,
		atomic: true,
	}
}

func positionedAttachmentPiece(
	role ir.ScriptRole,
	base, top, bottom, topLeft, bottomLeft, topRight, bottomRight piece,
) piece {
	fields := []struct {
		label  string
		speech string
		value  piece
	}{
		{"t", "top attachment", top},
		{"b", "bottom attachment", bottom},
		{"tl", "top-left attachment", topLeft},
		{"bl", "bottom-left attachment", bottomLeft},
		{"tr", "top-right attachment", topRight},
		{"br", "bottom-right attachment", bottomRight},
	}

	var text strings.Builder
	text.WriteString(positionedAttachmentName(role))
	text.WriteByte('(')
	text.WriteString(scriptBaseText(base))
	speechParts := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmedPieceText(field.value) == "" {
			continue
		}
		text.WriteString(", ")
		text.WriteString(field.label)
		text.WriteString(": ")
		text.WriteString(field.value.text)
		if trimmed := strings.TrimSpace(field.value.speech); trimmed != "" {
			speechParts = append(speechParts, field.speech+" "+trimmed)
		}
	}
	text.WriteByte(')')
	speech := strings.TrimSpace(base.speech)
	if len(speechParts) != 0 {
		if speech != "" {
			speech += " with "
		}
		speech += strings.Join(speechParts, ", ")
	}
	return piece{text: text.String(), speech: speech, atomic: true, word: base.word}
}

func positionedAttachmentName(role ir.ScriptRole) string {
	switch role {
	case ir.ScriptRoleLimits:
		return "limits"
	case ir.ScriptRoleOverlay:
		return "overlay"
	case ir.ScriptRoleBraceLabel:
		return "labeled"
	default:
		return "attach"
	}
}

func needsPositionedAttachment(
	role ir.ScriptRole,
	top, bottom, topLeft, bottomLeft, topRight, bottomRight piece,
) bool {
	switch role {
	case ir.ScriptRoleOrdinary:
		return (trimmedPieceText(top) != "" && trimmedPieceText(topRight) != "") ||
			(trimmedPieceText(bottom) != "" && trimmedPieceText(bottomRight) != "")
	case ir.ScriptRoleLimits:
		return false
	default:
		return trimmedPieceText(topLeft) != "" ||
			trimmedPieceText(bottomLeft) != "" ||
			trimmedPieceText(topRight) != "" ||
			trimmedPieceText(bottomRight) != ""
	}
}

func writeSideScripts(
	text *strings.Builder,
	base piece,
	topLeft, bottomLeft, bottom, top, topRight, bottomRight piece,
) {
	if trimmedPieceText(topLeft) != "" || trimmedPieceText(bottomLeft) != "" {
		text.WriteString("{}")
	}
	if trimmedPieceText(topLeft) != "" {
		text.WriteString(sideScriptText("^", topLeft))
	}
	if trimmedPieceText(bottomLeft) != "" {
		text.WriteString(sideScriptText("_", bottomLeft))
	}
	text.WriteString(scriptBaseText(base))
	if trimmedPieceText(bottom) != "" {
		text.WriteString(sideScriptText("_", bottom))
	}
	if trimmedPieceText(top) != "" {
		text.WriteString(sideScriptText("^", top))
	}
	if trimmedPieceText(bottomRight) != "" {
		text.WriteString(sideScriptText("_", bottomRight))
	}
	if trimmedPieceText(topRight) != "" {
		text.WriteString(sideScriptText("^", topRight))
	}
}

func appendSideScriptSpeech(speech string, topLeft, bottomLeft, topRight, bottomRight piece) string {
	if trimmedPieceText(bottomLeft) != "" {
		speech += " with pre-subscript " + bottomLeft.speech
	}
	if trimmedPieceText(topLeft) != "" {
		speech += " with pre-superscript " + topLeft.speech
	}
	if trimmedPieceText(bottomRight) != "" {
		speech += " sub " + bottomRight.speech
	}
	if trimmedPieceText(topRight) != "" {
		speech += powerSpeech(topRight)
	}
	return speech
}

func (r *renderer) fenced(n *ir.Fenced, env renderEnv, depth int) (piece, error) {
	var open, close piece
	var err error
	if n.Open != nil {
		open, err = r.render(n.Open, env, depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	body, err := r.renderList(n.Body, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	if n.Close != nil {
		close, err = r.render(n.Close, env, depth+1)
		if err != nil {
			return piece{}, err
		}
	}
	speech := joinSpeech(open.speech, body.speech, close.speech)
	return piece{
		text:   open.text + body.text + close.text,
		speech: speech,
		atomic: true,
	}, nil
}

func (r *renderer) accent(n *ir.Accent, env renderEnv, depth int) (piece, error) {
	body, err := r.render(n.Body, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	accent, err := r.render(n.Accent, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	name, speechName, known := accentName(accent.text)
	if !known {
		name = "accent"
		speechName = "accent " + safeSpeech(accent.speech)
	}
	if n.Position == ir.PositionBelow && !strings.HasPrefix(speechName, "under") {
		speechName += " below"
	}
	text := name + "(" + body.text + ")"
	if !known {
		text = "accent(" + accent.text + ", " + body.text + ")"
	}
	return piece{
		text:   text,
		speech: body.speech + " with " + speechName,
		atomic: true,
	}, nil
}

func (r *renderer) line(n *ir.Line, env renderEnv, depth int) (piece, error) {
	body, err := r.render(n.Body, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	name := "overline"
	speech := body.speech + " with overline"
	if n.Position == ir.PositionBelow {
		name = "underline"
		speech = body.speech + " with underline"
	}
	return piece{
		text:   name + "(" + body.text + ")",
		speech: speech,
		atomic: true,
	}, nil
}

func (r *renderer) cancel(n *ir.Cancel, env renderEnv, depth int) (piece, error) {
	body, err := r.render(n.Body, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	if n.Role == ir.CancelRoleNegation {
		if text, speech, ok := negatedRelation(body.text); ok {
			return piece{text: text, speech: speech, atomic: true}, nil
		}
		return piece{
			text:   "not(" + body.text + ")",
			speech: "not " + body.speech,
			atomic: true,
		}, nil
	}
	name := "cancel"
	speech := body.speech + " canceled"
	if n.Cross {
		name = "xcancel"
		speech = body.speech + " crossed out"
	} else if n.Inverted {
		name = "bcancel"
		speech = body.speech + " canceled with a backslash"
	}
	return piece{
		text:   name + "(" + body.text + ")",
		speech: speech,
		atomic: true,
	}, nil
}

func (r *renderer) table(n *ir.Table, env renderEnv, depth int) (piece, error) {
	rowText := make([]string, 0, len(n.Rows))
	rowSpeech := make([]string, 0, len(n.Rows))
	for i, row := range n.Rows {
		textParts := make([]string, 0, len(row.Cells))
		speechParts := make([]string, 0, len(row.Cells))
		emptyCells := 0
		for _, cell := range row.Cells {
			cellPiece, err := r.render(cell, env, depth+1)
			if err != nil {
				return piece{}, err
			}
			cellText := cellPiece.text
			cellSpeech := cellPiece.speech
			if strings.TrimSpace(cellText) == "" {
				emptyCells++
				cellText = "empty"
				cellSpeech = "empty cell"
			}
			textParts = append(textParts, cellText)
			speechParts = append(speechParts, cellSpeech)
		}
		text, err := joinBounded(textParts, ", ", r.limits.MaxTextBytes, "table text")
		if err != nil {
			return piece{}, err
		}
		speech, err := joinBounded(speechParts, ", ", r.limits.MaxTextBytes, "table speech")
		if err != nil {
			return piece{}, err
		}
		if text == "" {
			text = "empty"
			speech = "empty row"
		}
		if row.Tag != nil {
			tag, err := r.render(row.Tag, env, depth+1)
			if err != nil {
				return piece{}, err
			}
			text += " [tag: " + tag.text + "]"
			if len(text) > r.limits.MaxTextBytes {
				return piece{}, fmt.Errorf("%w: table text exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
			}
			speech = "tagged " + tag.speech + ": " + speech
		}
		if emptyCells == len(row.Cells) && len(row.Cells) > 0 {
			speech = "empty cells"
		}
		rowText = append(rowText, text)
		rowSpeech = append(rowSpeech, fmt.Sprintf("row %d: %s", i+1, speech))
	}
	text, err := joinBounded(rowText, "; ", r.limits.MaxTextBytes, "table text")
	if err != nil {
		return piece{}, err
	}
	speech, err := joinBounded(rowSpeech, "; ", r.limits.MaxTextBytes, "table speech")
	if err != nil {
		return piece{}, err
	}
	switch n.Role {
	case ir.TableRoleArray:
		return piece{
			text:   "array(" + text + ")",
			speech: fmt.Sprintf("array with %d rows: %s", len(n.Rows), speech),
			atomic: true,
		}, nil
	case ir.TableRoleCases:
		return piece{
			text:   "cases(" + text + ")",
			speech: fmt.Sprintf("piecewise definition with %d rows: %s", len(n.Rows), speech),
			atomic: true,
		}, nil
	case ir.TableRoleVector:
		return piece{
			text:   "vector(" + text + ")",
			speech: fmt.Sprintf("column vector with %d entries: %s", len(n.Rows), speech),
			atomic: true,
		}, nil
	default:
		return piece{
			text:   "matrix(" + text + ")",
			speech: fmt.Sprintf("matrix with %d rows: %s", len(n.Rows), speech),
			atomic: true,
		}, nil
	}
}

func (r *renderer) multiline(n *ir.Multiline, env renderEnv, depth int) (piece, error) {
	rowText := make([]string, 0, len(n.Rows))
	rowSpeech := make([]string, 0, len(n.Rows))
	for i, row := range n.Rows {
		part, err := r.renderList(row, env, depth+1)
		if err != nil {
			return piece{}, err
		}
		text := part.text
		speech := part.speech
		if strings.TrimSpace(text) == "" {
			text = "empty"
			speech = "empty row"
		}
		rowText = append(rowText, text)
		rowSpeech = append(rowSpeech, fmt.Sprintf("row %d: %s", i+1, speech))
	}
	text, err := joinBounded(rowText, "; ", r.limits.MaxTextBytes, "multiline text")
	if err != nil {
		return piece{}, err
	}
	speech, err := joinBounded(rowSpeech, "; ", r.limits.MaxTextBytes, "multiline speech")
	if err != nil {
		return piece{}, err
	}
	label := "multiline expression"
	if n.Centered {
		label = "centered multiline expression"
	}
	return piece{
		text:   "multiline(" + text + ")",
		speech: label + ": " + speech,
		atomic: true,
	}, nil
}

func (r *renderer) style(n *ir.Style, env renderEnv, depth int) (piece, error) {
	childEnv := env
	if n.SetSize {
		childEnv.display = n.NewSize == ir.SizeDisplay
	}
	return r.child(n.Body, childEnv, depth)
}

func (r *renderer) child(node ir.Node, env renderEnv, depth int) (piece, error) {
	return r.render(node, env, depth+1)
}

func (r *renderer) box(n *ir.Box, env renderEnv, depth int) (piece, error) {
	body, err := r.render(n.Body, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	if !n.HasBorder && !n.HasFill {
		return body, nil
	}
	var speech string
	var text string
	if strings.TrimSpace(body.text) == "" {
		switch {
		case n.HasBorder && n.HasFill:
			speech = "filled and bordered rectangle"
			text = "filled-rectangle-with-border"
		case n.HasBorder:
			speech = "rectangle outline"
			text = "rectangle"
		default:
			speech = "filled rectangle"
			text = "filled-rectangle"
		}
	} else {
		text = "box(" + body.text + ")"
		switch {
		case n.HasBorder && n.HasFill:
			speech = "filled and bordered box containing " + body.speech
		case n.HasBorder:
			speech = "boxed " + body.speech
		default:
			speech = "filled box containing " + body.speech
		}
	}
	return piece{
		text:   text,
		speech: speech,
		atomic: true,
	}, nil
}

func (r *renderer) phantom(n *ir.Phantom, env renderEnv, depth int) (piece, error) {
	body, err := r.render(n.Body, env, depth+1)
	if err != nil {
		return piece{}, err
	}
	return piece{
		text:   "phantom(" + body.text + ")",
		speech: "phantom of " + body.speech,
		atomic: true,
	}, nil
}

func (r *renderer) renderList(nodes []ir.Node, env renderEnv, depth int) (piece, error) {
	var text strings.Builder
	var speech strings.Builder
	haveSpeech := false
	lastSpace := false
	lastWord := false
	lastText := ""
	atomic := false
	rendered := 0
	for _, node := range nodes {
		part, err := r.render(node, env, depth)
		if err != nil {
			return piece{}, err
		}
		if text.Len()+len(part.text) > r.limits.MaxTextBytes {
			return piece{}, fmt.Errorf("%w: text output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
		}
		if needsTextBoundary(lastText, lastWord, part) {
			if text.Len()+1 > r.limits.MaxTextBytes {
				return piece{}, fmt.Errorf("%w: text output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
			}
			text.WriteByte(' ')
		}
		text.WriteString(part.text)
		if err := r.appendSpeech(&speech, &haveSpeech, &lastSpace, part); err != nil {
			return piece{}, err
		}
		trimmed := strings.TrimSpace(part.text)
		if trimmed != "" {
			rendered++
			atomic = part.atomic
			lastWord = part.word
		} else if part.text != "" {
			lastWord = false
		}
		if part.text != "" {
			lastText = part.text
		}
	}
	return piece{
		text:   text.String(),
		speech: speech.String(),
		atomic: rendered == 1 && atomic,
		word:   rendered == 1 && lastWord,
	}, nil
}

func (r *renderer) leaf(text, speech string, variant ir.Variant) piece {
	out := piece{text: text, speech: speech, atomic: true}
	name, spoken := variantName(variant)
	if name == "" {
		return out
	}
	out.text = name + "(" + text + ")"
	if spoken != "" {
		out.speech = spoken + " " + speech
	}
	return out
}

func (r *renderer) bounded(in piece) (piece, error) {
	if !utf8.ValidString(in.text) || !utf8.ValidString(in.speech) {
		return piece{}, fmt.Errorf("%w: alternative text is not valid UTF-8", ErrInvalidRoot)
	}
	if len(in.text) > r.limits.MaxTextBytes {
		return piece{}, fmt.Errorf("%w: text output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
	}
	if len(in.speech) > r.limits.MaxTextBytes {
		return piece{}, fmt.Errorf("%w: speech output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
	}
	return in, nil
}

func (r *renderer) appendSpeech(out *strings.Builder, have, lastSpace *bool, part piece) error {
	if part.speech == "" {
		return nil
	}
	next := part.speech
	if *have && !part.attach &&
		!*lastSpace &&
		!strings.HasPrefix(next, " ") {
		next = " " + next
	}
	if out.Len()+len(next) > r.limits.MaxTextBytes {
		return fmt.Errorf("%w: speech output exceeds %d bytes", ErrLimitExceeded, r.limits.MaxTextBytes)
	}
	out.WriteString(next)
	*have = true
	*lastSpace = strings.HasSuffix(next, " ")
	return nil
}

func (r *renderer) note(err error) {
	if r.firstErr == nil {
		r.firstErr = err
	}
}

func scriptEnv(base ir.Base, env renderEnv) renderEnv {
	if base.Size != ir.SizeInherit {
		env.display = base.Size == ir.SizeDisplay
	}
	return env
}

func appendScriptPiece(first, second piece) piece {
	switch {
	case strings.TrimSpace(second.text) == "":
		return first
	case strings.TrimSpace(first.text) == "":
		return second
	default:
		return piece{
			text:   first.text + " " + second.text,
			speech: joinSpeech(first.speech, second.speech),
			atomic: true,
		}
	}
}

func scriptBaseText(base piece) string {
	if base.atomic {
		return base.text
	}
	return "(" + base.text + ")"
}

func sideScriptText(mark string, script piece) string {
	if strings.TrimSpace(script.text) == "" {
		return ""
	}
	if mark == "^" {
		if superscript, ok := toSuperscript(script.text); ok && script.atomic {
			return superscript
		}
	}
	if script.atomic {
		return mark + script.text
	}
	return mark + "{" + script.text + "}"
}

func powerSpeech(script piece) string {
	switch strings.TrimSpace(script.text) {
	case "2":
		return " squared"
	case "3":
		return " cubed"
	default:
		return " to the power " + script.speech
	}
}

func groupedText(in piece) string {
	if in.atomic {
		return in.text
	}
	return "(" + in.text + ")"
}

func spacePiece(n *ir.Space) piece {
	switch n.Type {
	case ir.SpaceIgnored:
		return piece{}
	case ir.SpaceInterword:
		return piece{text: " ", speech: " ", attach: true}
	default:
		text, speech := explicitSpace(n.Amount)
		return piece{text: text, speech: speech}
	}
}

func explicitSpace(length ir.Length) (string, string) {
	if length.IsZero() {
		return "", "zero space"
	}
	const scale = 1_000_000
	switch {
	case length.Unit == ir.LengthMu && length.Value == 3*scale:
		return "\u2009", "thin space"
	case length.Unit == ir.LengthMu && length.Value == 4*scale:
		return "\u205f", "medium space"
	case length.Unit == ir.LengthMu && length.Value == 5*scale:
		return "\u2005", "thick space"
	case length.Unit == ir.LengthMu && length.Value == 9*scale:
		return "\u2002", "en space"
	case length.Unit == ir.LengthMu && length.Value == 18*scale:
		return "\u2003", "quad space"
	case length.Unit == ir.LengthMu && length.Value == 36*scale:
		return "\u2003\u2003", "double quad space"
	case length.Unit == ir.LengthMu && length.Value == -3*scale:
		return "space(-3mu)", "negative thin space"
	case length.Unit == ir.LengthMu && length.Value == -4*scale:
		return "space(-4mu)", "negative medium space"
	case length.Unit == ir.LengthMu && length.Value == -5*scale:
		return "space(-5mu)", "negative thick space"
	case length.Unit == ir.LengthEm && length.Value == 500_000:
		return "\u2002", "en space"
	case length.Unit == ir.LengthEm && length.Value == scale:
		return "\u2003", "quad space"
	case length.Unit == ir.LengthEm && length.Value == 2*scale:
		return "\u2003\u2003", "double quad space"
	default:
		value := strconv.FormatFloat(length.Float(), 'f', -1, 64)
		return "space(" + value + lengthUnitName(length.Unit) + ")", "explicit space"
	}
}

func lengthUnitName(unit ir.LengthUnit) string {
	switch unit {
	case ir.LengthEm:
		return "em"
	case ir.LengthEx:
		return "ex"
	case ir.LengthMu:
		return "mu"
	case ir.LengthPt:
		return "pt"
	case ir.LengthDip:
		return "dip"
	case ir.LengthPercent:
		return "%"
	default:
		return "unit"
	}
}

func accentName(mark string) (textName, speechName string, ok bool) {
	mark = strings.TrimSpace(mark)
	if mark == "" {
		return "", "", false
	}
	for _, r := range mark {
		switch r {
		case '\u0302', '^', 'ˆ', '⌃':
			return "hat", "hat", true
		case '\u0304', '¯', '-', '−', '―':
			return "bar", "bar", true
		case '\u0303', '~', '˜', '∼':
			return "tilde", "tilde", true
		case '\u0307', '.', '⋅':
			return "dot", "dot", true
		case '\u0308', '¨':
			return "double-dot", "double dot", true
		case '\u20d7', '→':
			return "vector", "vector arrow", true
		case '\u0301', '´':
			return "acute", "acute accent", true
		case '\u0300', '`':
			return "grave", "grave accent", true
		case '\u0306', '˘':
			return "breve", "breve", true
		case '\u030c', 'ˇ':
			return "caron", "caron", true
		case '\u030a', '∘', '○':
			return "ring", "ring", true
		case '\u0327', '¸':
			return "cedilla", "cedilla", true
		case '\u0328', '˛':
			return "ogonek", "ogonek", true
		case '\u030b', '˝':
			return "double-acute", "double acute accent", true
		case '⏞':
			return "overbrace", "overbrace", true
		case '⏟':
			return "underbrace", "underbrace", true
		case '⎴':
			return "overbracket", "overbracket", true
		case '⎵':
			return "underbracket", "underbracket", true
		case '⏜':
			return "overparen", "overparen", true
		case '⏝':
			return "underparen", "underparen", true
		case '⏠':
			return "overshell", "overshell", true
		case '⏡':
			return "undershell", "undershell", true
		}
	}
	return "", "", false
}

func toSuperscript(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	var out strings.Builder
	for _, r := range text {
		replacement, ok := superscripts[r]
		if !ok {
			return "", false
		}
		out.WriteRune(replacement)
	}
	return out.String(), true
}

var superscripts = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
	'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾',
	'n': 'ⁿ', 'i': 'ⁱ',
}

func variantName(variant ir.Variant) (string, string) {
	switch variant {
	case ir.VariantBold:
		return "bold", "bold"
	case ir.VariantBoldItalic:
		return "bold-italic", "bold italic"
	case ir.VariantDoubleStruck:
		return "double-struck", "double-struck"
	case ir.VariantScript:
		return "script", "script"
	case ir.VariantBoldScript:
		return "bold-script", "bold script"
	case ir.VariantFraktur:
		return "fraktur", "fraktur"
	case ir.VariantBoldFraktur:
		return "bold-fraktur", "bold fraktur"
	case ir.VariantSans:
		return "sans-serif", "sans-serif"
	case ir.VariantBoldSans:
		return "bold-sans-serif", "bold sans-serif"
	case ir.VariantMonospace:
		return "monospace", "monospace"
	default:
		return "", ""
	}
}

func marker(message string) piece {
	return piece{
		text:   "[" + message + "]",
		speech: message,
		atomic: true,
	}
}

func trimmedPieceText(piece piece) string {
	return strings.TrimSpace(piece.text)
}

func isWordText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	sawWord := false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			sawWord = true
		case unicode.IsSpace(r):
		default:
			return false
		}
	}
	return sawWord
}

func needsTextBoundary(lastText string, lastWord bool, next piece) bool {
	if lastText == "" || next.text == "" || !lastWord && !next.word {
		return false
	}
	if endsWithSpace(lastText) || startsWithSpace(next.text) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(next.text)
	return !strings.ContainsRune("([{⟨⌈⌊)]}⟩⌉⌋,.;:!?", first)
}

func startsWithSpace(text string) bool {
	first, _ := utf8.DecodeRuneInString(text)
	return unicode.IsSpace(first)
}

func endsWithSpace(text string) bool {
	last, _ := utf8.DecodeLastRuneInString(text)
	return unicode.IsSpace(last)
}

func negatedRelation(text string) (string, string, bool) {
	switch strings.TrimSpace(text) {
	case "=":
		return "≠", "not equal to", true
	case "∈":
		return "∉", "not an element of", true
	case "<":
		return "≮", "not less than", true
	case ">":
		return "≯", "not greater than", true
	case "≤":
		return "≰", "not less than or equal to", true
	case "≥":
		return "≱", "not greater than or equal to", true
	case "∼":
		return "≁", "not similar to", true
	case "≡":
		return "≢", "not equivalent to", true
	case "⊂":
		return "⊄", "not a subset of", true
	case "⊆":
		return "⊈", "not a subset of or equal to", true
	case "⊃":
		return "⊅", "not a superset of", true
	case "⊇":
		return "⊉", "not a superset of or equal to", true
	default:
		return "", "", false
	}
}

func failedAlt(err error, limit int) display.Alt {
	if limit <= 0 {
		limit = 1
	}
	message := "alternative text generation failed"
	if err != nil {
		detail := sanitizeFallbackText(err.Error())
		detail = truncateUTF8Bytes(detail, limit)
		message += ": " + detail
	}
	message = sanitizeFallbackText(message)
	return display.Alt{
		Text:   truncateUTF8Bytes(sanitizeFallbackText("["+message+"]"), limit),
		Speech: truncateUTF8Bytes(message, limit),
	}
}

func sanitizeFallbackText(text string) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	return strings.TrimSpace(text)
}

func truncateUTF8Bytes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	text = text[:limit]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

func joinSpeech(parts ...string) string {
	return joinSpeechWith(" ", parts...)
}

func joinSpeechWith(separator string, parts ...string) string {
	nonempty := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			nonempty = append(nonempty, trimmed)
		}
	}
	return strings.Join(nonempty, separator)
}

func joinBounded(parts []string, separator string, limit int, label string) (string, error) {
	total := 0
	for i, part := range parts {
		total += len(part)
		if i > 0 {
			total += len(separator)
		}
		if total > limit {
			return "", fmt.Errorf("%w: %s exceeds %d bytes", ErrLimitExceeded, label, limit)
		}
	}
	return strings.Join(parts, separator), nil
}

func safeSpeech(text string) string {
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		return trimmed
	}
	return "empty"
}

func nilNode(node ir.Node) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func nodeKindName(kind ir.NodeKind) string {
	switch kind {
	case ir.KindRow:
		return "row"
	case ir.KindGlyph:
		return "glyph"
	case ir.KindText:
		return "text"
	case ir.KindOperator:
		return "operator"
	case ir.KindSpace:
		return "space"
	case ir.KindFraction:
		return "fraction"
	case ir.KindRadical:
		return "radical"
	case ir.KindScripts:
		return "scripts"
	case ir.KindFenced:
		return "fenced"
	case ir.KindAccent:
		return "accent"
	case ir.KindLine:
		return "line"
	case ir.KindCancel:
		return "cancel"
	case ir.KindTable:
		return "table"
	case ir.KindMultiline:
		return "multiline"
	case ir.KindStyle:
		return "style"
	case ir.KindColor:
		return "color"
	case ir.KindBox:
		return "box"
	case ir.KindPhantom:
		return "phantom"
	case ir.KindError:
		return "error"
	case ir.KindPlaceholder:
		return "placeholder"
	case ir.KindBreak:
		return "break"
	case ir.KindAlignMarker:
		return "align-marker"
	default:
		return "kind-" + strconv.Itoa(int(kind))
	}
}
