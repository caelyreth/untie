package typst

import (
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) lowerMatrix(call *astNode, kind string) ir.Node {
	positional, ok := l.expandPositional(call)
	if !ok {
		return l.errNode(call.span, l.sourceText(call.span), source.CodeUnsupported)
	}
	if len(positional) > l.limits.MaxTableCells {
		l.errorAt(call.span, source.CodeLimitExceeded, "table cell limit exceeded")
		positional = positional[:l.limits.MaxTableCells]
	}

	grid := make([][]ir.Node, 0, 2)
	switch kind {
	case "mat":
		current := make([]ir.Node, 0, len(positional))
		for _, arg := range positional {
			for _, segment := range splitAlign(arg.node) {
				current = append(current, l.lowerCell(segment, false))
			}
			if arg.semicolonAfter {
				grid = append(grid, current)
				current = make([]ir.Node, 0, len(positional))
			}
		}
		if len(current) > 0 || len(grid) == 0 {
			grid = append(grid, current)
		}
	case "vec":
		for _, arg := range positional {
			chunks := splitLinebreaks(arg.node)
			for _, chunk := range chunks {
				grid = append(grid, []ir.Node{l.lowerCell(chunk, false)})
			}
		}
	case "cases":
		for _, arg := range positional {
			chunks := splitLinebreaks(arg.node)
			for _, chunk := range chunks {
				cells := make([]ir.Node, 0, 2)
				for _, segment := range splitAlign(chunk) {
					cells = append(cells, l.lowerCell(segment, false))
				}
				grid = append(grid, cells)
			}
		}
	}

	maxColumns := 0
	for _, row := range grid {
		maxColumns = max(maxColumns, len(row))
	}
	align := ir.AlignCenter
	if arg := namedArgument(call, "align"); arg != nil {
		if text, ok := l.valueString(arg.node); ok {
			switch text {
			case "left", "start":
				align = ir.AlignLeft
			case "right", "end":
				align = ir.AlignRight
			case "center", "horizon":
				align = ir.AlignCenter
			default:
				l.errorAt(arg.span, source.CodeInvalidValue, "unknown alignment %q", text)
			}
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected alignment, found %s", l.valueType(arg.node))
		}
	} else if kind == "cases" {
		align = ir.AlignLeft
	}

	rows := make([]ir.TableRow, len(grid))
	for i, cells := range grid {
		rows[i].Cells = cells
	}
	table := ir.NewTable(call.span, rows...)
	for range maxColumns {
		table.Columns = append(table.Columns, align)
	}
	switch kind {
	case "mat":
		table.RowGap = ir.EmLength(0.2)
		table.ColumnGap = ir.EmLength(0.5)
	case "vec":
		table.RowGap = ir.EmLength(0.2)
		table.ColumnGap = ir.EmLength(0)
	case "cases":
		table.RowGap = ir.EmLength(0.2)
		table.ColumnGap = ir.EmLength(0)
	}
	if arg := namedArgument(call, "gap"); arg != nil {
		if gap, ok := l.valueLength(arg.node); ok {
			table.RowGap = gap
			table.ColumnGap = gap
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected relative length, found %s", l.valueType(arg.node))
		}
	}
	if arg := namedArgument(call, "row-gap"); arg != nil {
		if gap, ok := l.valueLength(arg.node); ok {
			table.RowGap = gap
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected relative length, found %s", l.valueType(arg.node))
		}
	}
	if arg := namedArgument(call, "column-gap"); arg != nil {
		if gap, ok := l.valueLength(arg.node); ok {
			table.ColumnGap = gap
		} else {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected relative length, found %s", l.valueType(arg.node))
		}
	}
	if kind == "mat" {
		l.applyAugment(call, table, maxColumns)
	}

	openText, closeText := "(", ")"
	if kind == "cases" {
		openText, closeText = "{", "}"
	}
	if arg := namedArgument(call, "delim"); arg != nil {
		open, close, ok := l.valueDelimiters(arg.node)
		if !ok {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected delimiter, symbol, string, or two-element array")
		} else {
			openText, closeText = open, close
		}
	}
	if kind == "cases" {
		if arg := namedArgument(call, "reverse"); arg != nil {
			if reverse, ok := l.valueBool(arg.node); ok && reverse {
				openText, closeText = closeText, openText
			}
		}
	}

	var body ir.Node = table
	if openText != "" || closeText != "" {
		if closeText == "" {
			// The sealed IR currently treats a nil closing fence as an invalid
			// child. Preserve the visible one-sided delimiter without creating
			// malformed Fenced output.
			body = ir.NewRow(call.span)
			body.(*ir.Row).Items = []ir.Node{table, l.glyph(source.NewRange(call.span.Start, call.span.Start), openText)}
			return body
		}
		fenced := ir.NewFenced(call.span, table)
		if openText != "" {
			open := l.glyph(source.NewRange(call.span.Start, call.span.Start), openText)
			open.Class = ir.ClassOpening
			fenced.Open = open
		}
		if closeText != "" {
			closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), closeText)
			closeGlyph.Class = ir.ClassClosing
			fenced.Close = closeGlyph
		}
		fenced.Balanced = true
		fenced.Class = ir.ClassInner
		body = fenced
	}
	return body
}

func (l *lowerer) lowerCell(n *astNode, warnLinebreak bool) ir.Node {
	if n == nil {
		return ir.NewRow(source.Detached())
	}
	if warnLinebreak && containsLinebreak(n) {
		l.warningAt(n.span, source.CodeUnsupported, "linebreaks are ignored in matrix cells")
	}
	return l.lower(n)
}

func containsLinebreak(n *astNode) bool {
	if n == nil {
		return false
	}
	if n.kind == astLinebreak {
		return true
	}
	for _, item := range n.items {
		if containsLinebreak(item) {
			return true
		}
	}
	return false
}

func splitAlign(n *astNode) []*astNode {
	if n == nil {
		return []*astNode{nil}
	}
	if n.kind != astRow {
		return []*astNode{n}
	}
	hasAlign := false
	for _, item := range n.items {
		if item.kind == astAlign {
			hasAlign = true
			break
		}
	}
	if !hasAlign {
		return []*astNode{n}
	}
	out := make([]*astNode, 0, 3)
	current := make([]*astNode, 0, len(n.items))
	start := n.span.Start
	flush := func(end int) {
		row := newAST(astRow, source.NewRange(start, end))
		row.items = current
		out = append(out, row)
		current = make([]*astNode, 0, len(n.items))
	}
	for _, item := range n.items {
		if item.kind == astAlign {
			flush(item.span.Start)
			start = item.span.End
			continue
		}
		current = append(current, item)
	}
	flush(n.span.End)
	return out
}

func splitLinebreaks(n *astNode) []*astNode {
	if n == nil {
		return []*astNode{nil}
	}
	if n.kind != astRow {
		return []*astNode{n}
	}
	hasBreak := false
	for _, item := range n.items {
		if item.kind == astLinebreak {
			hasBreak = true
			break
		}
	}
	if !hasBreak {
		return []*astNode{n}
	}
	out := make([]*astNode, 0, 3)
	current := make([]*astNode, 0, len(n.items))
	start := n.span.Start
	flush := func(end int) {
		row := newAST(astRow, source.NewRange(start, end))
		row.items = current
		out = append(out, row)
		current = make([]*astNode, 0, len(n.items))
	}
	for _, item := range n.items {
		if item.kind == astLinebreak {
			flush(item.span.Start)
			start = item.span.End
			continue
		}
		current = append(current, item)
	}
	if len(current) > 0 {
		flush(n.span.End)
	}
	return out
}

func (l *lowerer) expandPositional(call *astNode) ([]astArg, bool) {
	out := make([]astArg, 0, len(call.args))
	for _, arg := range call.args {
		if arg.kind != argSpread {
			if arg.kind == argPositional {
				out = append(out, arg)
			}
			continue
		}
		v, ok := l.evalValue(arg.node)
		if !ok || v.kind != valueArray {
			l.errorAt(arg.span, source.CodeInvalidValue, "expected array in spread, found %s", l.valueType(arg.node))
			return nil, false
		}
		for _, item := range v.array {
			var node *astNode
			if item.kind == valueContent {
				node = item.content
			} else {
				node = newAST(astLiteral, arg.span)
				node.value = item
			}
			out = append(out, astArg{kind: argPositional, span: arg.span, node: node})
		}
	}
	return out, true
}

func (l *lowerer) valueDelimiters(n *astNode) (string, string, bool) {
	v, ok := l.evalValue(n)
	if !ok {
		return "", "", false
	}
	switch v.kind {
	case valueNone:
		return "", "", true
	case valueString, valueSymbol:
		open := v.text
		if v.kind == valueSymbol {
			open = v.symbol
		}
		if utf8.RuneCountInString(open) != 1 {
			return "", "", false
		}
		return open, inferClosingDelimiter(open), true
	case valueArray:
		if len(v.array) != 2 {
			return "", "", false
		}
		open, okOpen := delimiterValue(v.array[0])
		close, okClose := delimiterValue(v.array[1])
		return open, close, okOpen && okClose
	case valueContent:
		text := nodeText(l.lower(v.content))
		if utf8.RuneCountInString(text) != 1 {
			return "", "", false
		}
		return text, inferClosingDelimiter(text), true
	}
	return "", "", false
}

func delimiterValue(v value) (string, bool) {
	switch v.kind {
	case valueNone:
		return "", true
	case valueString:
		return v.text, utf8.RuneCountInString(v.text) == 1
	case valueSymbol:
		return v.symbol, utf8.RuneCountInString(v.symbol) == 1
	default:
		return "", false
	}
}

func inferClosingDelimiter(open string) string {
	if close := matchingDelimiter(open); close != "" {
		return close
	}
	pairs := map[string]string{
		")": "(", "]": "[", "}": "{", "⟩": "⟨", "⌋": "⌊", "⌉": "⌈", "⟧": "⟦",
	}
	if close, ok := pairs[open]; ok {
		return close
	}
	return open
}

func (l *lowerer) applyAugment(call *astNode, table *ir.Table, columns int) {
	arg := namedArgument(call, "augment")
	if arg == nil || isNoneAST(arg.node) {
		return
	}
	addLine := func(position int64) {
		if position < 0 {
			position = int64(columns) + position
		}
		if position < 0 || position > int64(columns) {
			l.errorAt(arg.span, source.CodeInvalidValue, "augmentation line outside table")
			return
		}
		table.Vertical = append(table.Vertical, ir.TableLine{
			Position: int(position),
			Count:    1,
		})
	}
	if position, ok := l.valueInteger(arg.node); ok {
		addLine(position)
		return
	}
	v, ok := l.evalValue(arg.node)
	if !ok || v.kind != valueDictionary {
		l.errorAt(arg.span, source.CodeInvalidValue, "expected integer or dictionary for augment")
		return
	}
	if vline, ok := v.entry("vline"); ok {
		switch vline.kind {
		case valueInteger:
			addLine(vline.integer)
		case valueArray:
			for _, item := range vline.array {
				if item.kind == valueInteger {
					addLine(item.integer)
				}
			}
		case valueNone:
		default:
			l.errorAt(arg.span, source.CodeInvalidValue, "expected integer or array for vline")
		}
	}
	if _, ok := v.entry("hline"); ok {
		l.warningAt(arg.span, source.CodeUnsupported, "horizontal augmentation lines are not represented by the matrix subset")
	}
}
