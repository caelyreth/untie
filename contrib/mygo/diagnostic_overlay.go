package untiemygo

import (
	"fmt"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

// DiagnosticMode selects optional editor diagnostics presentation.
type DiagnosticMode uint8

const (
	// DiagnosticUnderline draws marks below display areas covered by a
	// diagnostic source range.
	DiagnosticUnderline DiagnosticMode = 1 << iota
	// DiagnosticTooltip attaches a whole-element tooltip and accessible
	// description containing the diagnostic messages.
	DiagnosticTooltip
)

// DiagnosticFormat formats one diagnostic for a tooltip. It may be replaced
// by hosts that localize messages.
type DiagnosticFormat func(untie.Diagnostic) string

// DiagnosticMark is one diagnostic source range mapped to one box-local
// display area.
type DiagnosticMark struct {
	Diagnostic untie.Diagnostic
	Area       untie.Rect
}

// DiagnosticPainter paints one transformed diagnostic mark. Painters should
// only draw and must not mutate the box or the diagnostic.
type DiagnosticPainter func(painter *ui.Painter, mark DiagnosticMark)

// DiagnosticOptions configure editor diagnostics presentation.
//
// Underline uses DiagnosticPainter when set and otherwise draws a basic
// severity-colored underline. Tooltip uses Format when set and otherwise uses
// the diagnostic code and message.
type DiagnosticOptions struct {
	Mode   DiagnosticMode
	Format DiagnosticFormat
	Paint  DiagnosticPainter
}

// DiagnosticMarks maps diagnostics to their covered display areas in box-local
// coordinates. Diagnostics without valid covered source ranges produce no
// marks.
func DiagnosticMarks(
	box *untie.Box,
	diagnostics []untie.Diagnostic,
) []DiagnosticMark {
	if box == nil || len(diagnostics) == 0 {
		return nil
	}
	var marks []DiagnosticMark
	for _, diagnostic := range diagnostics {
		clone := diagnostic.Clone()
		for _, area := range box.Covering(diagnostic.Range) {
			marks = append(marks, DiagnosticMark{Diagnostic: clone, Area: area})
		}
	}
	return marks
}

// FormatDiagnostic returns the default English diagnostic tooltip text.
func FormatDiagnostic(diagnostic untie.Diagnostic) string {
	if diagnostic.Code == "" {
		return diagnostic.Message
	}
	return fmt.Sprintf("%s: %s", diagnostic.Code, diagnostic.Message)
}

// DiagnosticTooltipText joins diagnostics for a tooltip or accessible
// description. A custom formatter may localize each entry.
func DiagnosticTooltipText(
	diagnostics []untie.Diagnostic,
	format DiagnosticFormat,
) string {
	if len(diagnostics) == 0 {
		return ""
	}
	if format == nil {
		format = FormatDiagnostic
	}
	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		line := strings.TrimSpace(format(diagnostic))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func paintDiagnostics(
	painter *ui.Painter,
	viewport ui.Rect,
	box *untie.Box,
	diagnostics []untie.Diagnostic,
	draw DrawOptions,
	options DiagnosticOptions,
) {
	if painter == nil || options.Mode&DiagnosticUnderline == 0 {
		return
	}
	transform, err := drawTransform(box, ui.Rect{
		X: viewport.X,
		Y: viewport.Y,
		W: viewport.W,
		H: viewport.H,
	}, draw)
	if err != nil {
		return
	}
	for _, mark := range DiagnosticMarks(box, diagnostics) {
		area := transform.ApplyRect(mark.Area)
		mark.Area = area
		if options.Paint != nil {
			options.Paint(painter, mark)
			continue
		}
		paintDefaultDiagnostic(painter, mark)
	}
}

func paintDefaultDiagnostic(painter *ui.Painter, mark DiagnosticMark) {
	theme := painter.Theme()
	if theme == nil {
		theme = ui.LightTheme()
	}
	color := theme.TextMuted
	switch mark.Diagnostic.Severity {
	case untie.SeverityError:
		color = theme.Danger
	case untie.SeverityWarning:
		color = theme.Warning
	}
	y := mark.Area.Y + mark.Area.Height + max(1, theme.Space(0.25))
	painter.Fill(ui.Rect{
		X: mark.Area.X,
		Y: y,
		W: max(1, mark.Area.Width),
		H: max(1, theme.Space(0.25)),
	}, color, 0)
}
