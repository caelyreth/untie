package tex

import (
	"strconv"
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var matrixEnvironments = map[string]struct {
	open      string
	close     string
	size      ir.MathSize
	casesLike bool
}{
	"matrix":       {},
	"matrix*":      {},
	"smallmatrix":  {size: ir.SizeScript},
	"smallmatrix*": {size: ir.SizeScript},
	"pmatrix":      {open: "(", close: ")"},
	"pmatrix*":     {open: "(", close: ")"},
	"bmatrix":      {open: "[", close: "]"},
	"bmatrix*":     {open: "[", close: "]"},
	"Bmatrix":      {open: "{", close: "}"},
	"Bmatrix*":     {open: "{", close: "}"},
	"vmatrix":      {open: "|", close: "|"},
	"vmatrix*":     {open: "|", close: "|"},
	"Vmatrix":      {open: "‖", close: "‖"},
	"Vmatrix*":     {open: "‖", close: "‖"},
	"cases":        {open: "{", casesLike: true},
	"dcases":       {open: "{", size: ir.SizeDisplay, casesLike: true},
	"rcases":       {close: "}", casesLike: true},
	"drcases":      {close: "}", size: ir.SizeDisplay, casesLike: true},
}

var (
	defaultMatrixRowGap    = ir.RawLength(ir.LengthEm, 200_000)
	defaultMatrixColumnGap = ir.RawLength(ir.LengthEm, 500_000)
)

func (p *parser) parseEnvironment(command token) astNode {
	name, nameSpan := p.consumeEnvironmentName(command)
	if name == "" {
		return astErr(command.span, command.text, source.CodeInvalidEnvironment)
	}

	p.exp.beginScope()
	defer p.exp.endScope()

	if env, ok := matrixEnvironments[name]; ok {
		spec := p.consumeMatrixAlignment(nameSpan)
		if env.casesLike {
			spec = &arraySpec{columns: []ir.ColumnAlign{ir.AlignLeft, ir.AlignLeft}}
		}
		table := p.parseTableEnvironment(name, command, nameSpan, spec)
		if env.size != ir.SizeInherit {
			table.size = env.size
		}
		if env.open == "" && env.close == "" {
			return table
		}
		span := table.base().span
		var open astNode
		var close astNode
		if env.open != "" {
			open = newGlyphAST(nameSpan, env.open, ir.ClassOpening)
		}
		if env.close != "" {
			close = newGlyphAST(nameSpan, env.close, ir.ClassClosing)
		}
		return &astFencedNode{
			astBase:  newASTBaseAt(astFenced, span, tokenOrigin(command)),
			open:     open,
			close:    close,
			body:     []astNode{table},
			balanced: true,
			size:     env.size,
		}
	}

	switch name {
	case "array", "darray", "subarray":
		spec, specSpan, ok := p.consumeRawGroupTextChecked(command)
		if !ok {
			p.report(source.Diagnostic{
				Range:    normalizeRange(specSpan),
				Severity: source.SeverityError,
				Code:     source.CodeMissingArgument,
				Message:  "array environment requires a column specification",
			})
		}
		columns, vertical := p.parseArraySpec(nameSpan, spec)
		table := p.parseTableEnvironment(name, command, nameSpan, &arraySpec{columns: columns, vertical: vertical})
		switch name {
		case "darray":
			table.size = ir.SizeDisplay
		case "subarray":
			table.size = ir.SizeScript
		}
		return table
	case "aligned", "gathered", "split", "alignedat", "alignat", "alignat*",
		"gather", "gather*", "align", "align*":
		if name == "alignedat" || name == "alignat" || name == "alignat*" {
			countText, countSpan, ok := p.consumeRawGroupTextChecked(command)
			if !ok {
				p.report(source.Diagnostic{
					Range:    normalizeRange(countSpan),
					Severity: source.SeverityError,
					Code:     source.CodeMissingArgument,
					Message:  name + " environment requires a column count",
				})
			}
			if _, err := strconv.Atoi(strings.TrimSpace(countText)); err != nil {
				p.report(source.Diagnostic{
					Range:    normalizeRange(nameSpan),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidValue,
					Message:  "number of alignment columns must be a positive integer",
				})
			}
			return p.parseMultilineEnvironment(name, command, nameSpan)
		}
		return p.parseMultilineEnvironment(name, command, nameSpan)
	case "equation", "equation*":
		return p.parseMultilineEnvironment(name, command, nameSpan)
	case "CD":
		p.report(source.Diagnostic{
			Range:    normalizeRange(nameSpan),
			Severity: source.SeverityError,
			Code:     source.CodeUnsupported,
			Message:  "commutative-diagram (CD) environments are outside the neutral math IR",
		})
		endSpan := p.skipEnvironment(name)
		node := astErr(source.Merge(command.span, endSpan), "\\begin{"+name+"}", source.CodeUnsupported)
		return node
	default:
		p.report(source.Diagnostic{
			Range:    normalizeRange(nameSpan),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidEnvironment,
			Message:  "unknown TeX environment '" + name + "'",
		})
		endSpan := p.skipEnvironment(name)
		node := astErr(source.Merge(command.span, endSpan), "\\begin{"+name+"}", source.CodeInvalidEnvironment)
		return node
	}
}

func (p *parser) consumeMatrixAlignment(span source.Range) *arraySpec {
	alignText, alignSpan, ok := p.consumeRawOptionalGroup()
	if !ok {
		return nil
	}
	align := ir.AlignRight
	switch strings.TrimSpace(alignText) {
	case "l":
		align = ir.AlignLeft
	case "c":
		align = ir.AlignCenter
	case "r":
	default:
		p.report(source.Diagnostic{
			Range:    normalizeRange(source.Merge(span, alignSpan)),
			Severity: source.SeverityError,
			Code:     source.CodeInvalidValue,
			Message:  "matrix alignment must be 'l', 'c', or 'r'",
		})
	}
	return &arraySpec{align: &align}
}

func (p *parser) consumeRawGroupTextChecked(owner token) (string, source.Range, bool) {
	text, _, open, ok := p.consumeRawGroupTextRange()
	if !ok {
		return "", normalizeRange(owner.span), false
	}
	return text, source.Merge(owner.span, open), true
}

type arraySpec struct {
	columns  []ir.ColumnAlign
	vertical []ir.TableLine
	align    *ir.ColumnAlign
}

func (p *parser) parseTableEnvironment(name string, command token, nameSpan source.Range, spec *arraySpec) *astTableNode {
	var rows []astTableRow
	var horizontal []ir.TableLine
	var currentRow []astNode
	cells := 0
	span := source.Merge(command.span, nameSpan)
	endFound := false

	finalizeRow := func() {
		if len(currentRow) == 0 {
			currentRow = append(currentRow, emptyASTRow(span))
		}
		rows = append(rows, astTableRow{cells: currentRow})
		currentRow = nil
	}

	for {
		if t := p.exp.peek(); t.isControl() && (t.text == "\\hline" || t.text == "\\hdashline") {
			p.exp.next()
			span = source.Merge(span, t.span)
			horizontal = appendTableStroke(horizontal, ir.TableLine{
				Position: len(rows),
				Count:    1,
				Dashed:   t.text == "\\hdashline",
			})
			continue
		}
		cell := p.parseSequence(stopCell)
		cells++
		if cells > p.limits.MaxTableCells {
			p.limit(normalizeRange(cell.base().span), "TeX table cell limit exceeded")
		} else {
			currentRow = append(currentRow, cell)
		}
		span = source.Merge(span, cell.base().span)

		t := p.exp.peek()
		switch {
		case t.is(tokAlignment):
			p.exp.next()
			span = source.Merge(span, t.span)
			continue
		case t.isControl() && isRowBreak(t.text):
			p.exp.next()
			span = source.Merge(span, t.span)
			p.consumeRowBreakOptions()
			finalizeRow()
			continue
		case t.isControl() && (t.text == "\\hline" || t.text == "\\hdashline"):
			// A rule after cell content is applied at the next row boundary.
		case t.isControl() && t.text == "\\end":
			endCommand := p.exp.next()
			endName, endNameSpan := p.consumeEnvironmentName(endCommand)
			span = source.Merge(span, endNameSpan)
			if endName != name {
				p.report(source.Diagnostic{
					Range:    normalizeRange(endNameSpan),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidEnvironment,
					Message:  "environment '" + name + "' closed by '\\end{" + endName + "}'",
				})
			}
			endFound = true
		case t.isEOF() || t.is(tokEndGroup):
			p.report(source.Diagnostic{
				Range:    normalizeRange(span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidEnvironment,
				Message:  "missing \\end{" + name + "}",
			})
		default:
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidEnvironment,
				Message:  "unexpected token in environment",
			})
			p.exp.next()
		}
		if endFound || t.isEOF() || t.is(tokEndGroup) {
			break
		}
	}
	if len(currentRow) > 0 || len(rows) == 0 {
		finalizeRow()
	}

	table := &astTableNode{
		astBase:   newASTBaseAt(astTable, span, tokenOrigin(command)),
		rows:      rows,
		rowGap:    defaultMatrixRowGap,
		columnGap: defaultMatrixColumnGap,
		role:      tableRoleForEnvironment(name),
	}
	if name == "cases" || name == "rcases" {
		table.columnGap = ir.Length{}
	}
	if name == "smallmatrix" {
		table.size = ir.SizeScript
	}
	if name == "cases" || name == "rcases" {
		table.columns = []ir.ColumnAlign{ir.AlignLeft, ir.AlignLeft}
	}
	if spec != nil {
		if len(spec.columns) != 0 {
			table.columns = append([]ir.ColumnAlign(nil), spec.columns...)
		}
		table.vertical = spec.vertical
	}
	maxColumns := 0
	for _, row := range rows {
		maxColumns = max(maxColumns, len(row.cells))
	}
	if len(table.columns) < maxColumns {
		align := ir.AlignCenter
		if spec != nil && spec.align != nil {
			align = *spec.align
		}
		for len(table.columns) < maxColumns {
			table.columns = append(table.columns, align)
		}
	}
	table.horizontal = horizontal
	return table
}

func tableRoleForEnvironment(name string) ir.TableRole {
	switch name {
	case "array", "darray", "subarray":
		return ir.TableRoleArray
	case "cases", "dcases", "rcases", "drcases":
		return ir.TableRoleCases
	default:
		return ir.TableRoleMatrix
	}
}

func appendTableStroke(lines []ir.TableLine, line ir.TableLine) []ir.TableLine {
	if len(lines) != 0 {
		last := &lines[len(lines)-1]
		if last.Position == line.Position && last.Dashed == line.Dashed && last.Thickness == line.Thickness {
			last.Count += line.Count
			return lines
		}
	}
	if line.Count <= 0 {
		line.Count = 1
	}
	return append(lines, line)
}

func (p *parser) parseMultilineEnvironment(name string, command token, nameSpan source.Range) *astMultilineNode {
	var logicalRows [][]astNode
	var row []astNode
	span := source.Merge(command.span, nameSpan)
	endFound := false

	finalize := func() {
		if len(row) == 0 {
			row = append(row, emptyASTRow(span))
		}
		logicalRows = append(logicalRows, row)
		row = nil
	}
	for {
		cell := p.parseSequence(stopCell)
		row = append(row, cell)
		span = source.Merge(span, cell.base().span)
		t := p.exp.peek()
		switch {
		case t.is(tokAlignment):
			p.exp.next()
			row = append(row, &astAlignNode{astBase: newASTBaseAt(astAlign, t.span, tokenOrigin(t))})
			span = source.Merge(span, t.span)
			continue
		case t.isControl() && isRowBreak(t.text):
			p.exp.next()
			span = source.Merge(span, t.span)
			p.consumeRowBreakOptions()
			finalize()
			continue
		case t.isControl() && t.text == "\\end":
			endCommand := p.exp.next()
			endName, endNameSpan := p.consumeEnvironmentName(endCommand)
			span = source.Merge(span, endNameSpan)
			if endName != name {
				p.report(source.Diagnostic{
					Range:    normalizeRange(endNameSpan),
					Severity: source.SeverityError,
					Code:     source.CodeInvalidEnvironment,
					Message:  "environment '" + name + "' closed by '\\end{" + endName + "}'",
				})
			}
			endFound = true
		case t.isEOF() || t.is(tokEndGroup):
			p.report(source.Diagnostic{
				Range:    normalizeRange(span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidEnvironment,
				Message:  "missing \\end{" + name + "}",
			})
		default:
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidEnvironment,
				Message:  "unexpected token in environment",
			})
			p.exp.next()
		}
		if endFound || t.isEOF() || t.is(tokEndGroup) {
			break
		}
	}
	if len(row) > 0 || len(logicalRows) == 0 {
		finalize()
	}

	return &astMultilineNode{
		astBase:  newASTBaseAt(astMultiline, span, tokenOrigin(command)),
		rows:     logicalRows,
		centered: true,
	}
}

func (p *parser) consumeEnvironmentName(command token) (string, source.Range) {
	p.exp.consumeSpaces()
	open := p.exp.popRaw()
	if !open.is(tokBeginGroup) {
		p.exp.push(open)
		p.report(source.Diagnostic{
			Range:    normalizeRange(command.span),
			Severity: source.SeverityError,
			Code:     source.CodeMissingArgument,
			Message:  command.text + " requires an environment name in braces",
		})
		return "", command.span
	}
	var b strings.Builder
	span := source.Merge(command.span, open.span)
	for {
		t := p.exp.popRaw()
		if t.isEOF() {
			p.report(source.Diagnostic{
				Range:    normalizeRange(span),
				Severity: source.SeverityError,
				Code:     source.CodeUnclosedGroup,
				Message:  "unclosed environment name",
			})
			return strings.TrimSpace(b.String()), span
		}
		span = source.Merge(span, t.span)
		if t.is(tokEndGroup) {
			return strings.TrimSpace(b.String()), span
		}
		if t.isControl() || t.is(tokBeginGroup) {
			p.report(source.Diagnostic{
				Range:    normalizeRange(t.span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidEnvironment,
				Message:  "environment name must contain only plain characters",
			})
		}
		b.WriteString(t.text)
	}
}

func (p *parser) parseArraySpec(span source.Range, spec string) ([]ir.ColumnAlign, []ir.TableLine) {
	var columns []ir.ColumnAlign
	var vertical []ir.TableLine
	for _, r := range spec {
		switch r {
		case 'l':
			columns = append(columns, ir.AlignLeft)
		case 'c':
			columns = append(columns, ir.AlignCenter)
		case 'r':
			columns = append(columns, ir.AlignRight)
		case '|', ':':
			vertical = append(vertical, ir.TableLine{
				Position: len(columns),
				Count:    1,
				Dashed:   r == ':',
			})
		case ' ', '\t', '\r', '\n':
		default:
			p.report(source.Diagnostic{
				Range:    normalizeRange(span),
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "unsupported array column specifier '" + string(r) + "'",
			})
		}
	}
	return columns, vertical
}

func (p *parser) consumeRowBreakOptions() {
	p.exp.consumeSpaces()
	t := p.exp.peek()
	if !t.is(tokChar) {
		return
	}
	if t.text == "*" {
		p.exp.next()
		return
	}
	if t.text == "[" {
		p.parseOptionalArgument(token{text: "\\\\", span: t.span, origin: t.origin})
	}
}

func (p *parser) skipEnvironment(name string) source.Range {
	depth := 1
	span := source.Detached()
	for {
		t := p.exp.next()
		if t.isEOF() {
			return source.Merge(span, t.span)
		}
		span = source.Merge(span, t.span)
		if !t.isControl() {
			continue
		}
		switch t.text {
		case "\\begin":
			nested, nestedSpan := p.consumeEnvironmentName(t)
			span = source.Merge(span, nestedSpan)
			if nested == name {
				depth++
			}
		case "\\end":
			nested, nestedSpan := p.consumeEnvironmentName(t)
			span = source.Merge(span, nestedSpan)
			if nested == name {
				depth--
				if depth == 0 {
					return span
				}
			}
		}
	}
}

func emptyASTRow(span source.Range) astNode {
	return &astRowNode{astBase: newASTBase(astRow, span)}
}

func isRowBreak(command string) bool {
	switch command {
	case "\\\\", "\\cr", "\\crcr", "\\tabularnewline", "\\newline":
		return true
	default:
		return false
	}
}
