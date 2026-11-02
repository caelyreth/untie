package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

const (
	dialectTeX = iota
	dialectTypst
)

const (
	initialTeX = `\frac{-b \pm \sqrt{b^2 - 4ac}}{2a}`

	initialTypst = `(-b plus.minus sqrt(b^2 - 4 a c)) / (2 a)`
)

// app is the complete example state. MyGo calls view on the main thread after
// input and whenever the window needs another frame.
type app struct {
	engine  *untie.Engine
	dialect int

	texSource   string
	typstSource string

	editorHeight float32
	box          *untie.Box
	diagnostics  []untie.Diagnostic
	message      string
}

func newApp(fontBytes []byte) (*app, error) {
	if len(fontBytes) == 0 {
		return nil, errors.New("empty math font")
	}
	fonts, err := untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: "newcm-math"},
	})
	if err != nil {
		return nil, err
	}
	engine, err := untie.NewEngine(fonts, untie.Limits{})
	if err != nil {
		return nil, err
	}
	a := &app{
		engine:       engine,
		texSource:    initialTeX,
		typstSource:  initialTypst,
		editorHeight: 300,
	}
	a.compile()
	return a, nil
}

func (a *app) view(c *ui.Context) {
	ui.SplitVertical(c, &a.editorHeight, func() {
		a.editor(c)
	}, func() {
		a.preview(c)
	}).Fill()
}

func (a *app) editor(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Padding(12).Gap(10).Children(func() {
		ui.Row(c).Height(30).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Source").Bold()
			if ui.Segmented(c, &a.dialect, "TeX", "Typst").Label("Input syntax").Changed() {
				a.compile()
			}
			ui.Textf(c, "%d bytes", len(*a.source())).Grow(1).
				TextAlign(ui.End).TextColor(t.TextMuted).FontSize(12)
		})
		if ui.TextArea(c, a.source()).Label("Source").Font("monospace").
			FontSize(14).Padding(14).Grow(1).FillHeight().
			Background(t.Background).Radius(10).Border(1, t.Border).Changed() {
			a.compile()
		}
	})
}

func (a *app) preview(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Height(48).Padding(12, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Preview").Bold()
			ui.Text(c, a.dialectName()).FontSize(12).TextColor(t.Accent)
			if summary := a.diagnosticSummary(); summary != "" {
				color := t.TextMuted
				if a.hasDiagnosticErrors() {
					color = t.Danger
				}
				ui.Text(c, summary).FontSize(12).TextColor(color)
			}
			ui.Text(c, "Display").FontSize(12).Grow(1).
				TextAlign(ui.End).TextColor(t.TextMuted)
		})
		ui.Box(c).Height(1).Background(t.Border)
		ui.Box(c).Grow(1).Fill().Clip().Draw(func(p *ui.Painter, r ui.Rect) {
			if a.box == nil {
				drawMessage(p, r, a.message, t.Danger)
				return
			}
			drawMath(p, r, a.box)
		})
	})
}

func (a *app) source() *string {
	if a.dialect == dialectTypst {
		return &a.typstSource
	}
	return &a.texSource
}

func (a *app) dialectName() string {
	if a.dialect == dialectTypst {
		return "Typst"
	}
	return "TeX"
}

func (a *app) compile() {
	var expr *untie.Expr
	if a.dialect == dialectTypst {
		expr = untie.ParseTypst(*a.source(), untie.Display, untie.ParseOptions{})
	} else {
		expr = untie.ParseTeX(*a.source(), untie.Display, untie.ParseOptions{})
	}

	a.diagnostics = expr.Diagnostics()
	a.message = diagnosticMessage(a.diagnostics)
	box, err := a.engine.Layout(expr, untie.Style{Size: 32})
	if err != nil {
		a.box = nil
		a.message = err.Error()
		return
	}
	a.box = box
}

func (a *app) diagnosticSummary() string {
	errors, warnings := 0, 0
	for _, diagnostic := range a.diagnostics {
		switch diagnostic.Severity {
		case untie.SeverityError:
			errors++
		case untie.SeverityWarning:
			warnings++
		}
	}
	switch {
	case errors == 1:
		return "1 error"
	case errors > 1:
		return fmt.Sprintf("%d errors", errors)
	case warnings == 1:
		return "1 warning"
	case warnings > 1:
		return fmt.Sprintf("%d warnings", warnings)
	default:
		return ""
	}
}

func (a *app) hasDiagnosticErrors() bool {
	for _, diagnostic := range a.diagnostics {
		if diagnostic.Severity == untie.SeverityError {
			return true
		}
	}
	return false
}

func diagnosticMessage(diagnostics []untie.Diagnostic) string {
	var messages []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == untie.SeverityError {
			messages = append(messages, diagnostic.Message)
		}
	}
	return strings.Join(messages, "\n")
}
