package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/contrib/mygo"
	"github.com/caelyreth/untie/render"
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
	renderers [2]*untiemygo.Renderer
	dialect   int

	texSource   string
	typstSource string

	editorHeight float32
	diagnostics  []untie.Diagnostic
	message      string
	ready        bool
}

func newApp(fontBytes []byte) (*app, error) {
	if len(fontBytes) == 0 {
		return nil, errors.New("empty math font")
	}
	fonts, err := untie.LoadMathFont(fontBytes, untie.WithFontID("newcm-math"))
	if err != nil {
		return nil, err
	}
	newRenderer := func(dialect untie.Dialect) (*untiemygo.Renderer, error) {
		return untiemygo.New(fonts, untiemygo.Options{
			Dialect:      dialect,
			BaseSize:     16,
			DisplaySize:  32,
			CacheEntries: 128,
			Draw: untiemygo.DrawOptions{
				Padding: 16,
				Align:   render.AlignCenter,
			},
		})
	}
	texRenderer, err := newRenderer(untie.TeX)
	if err != nil {
		return nil, err
	}
	typstRenderer, err := newRenderer(untie.Typst)
	if err != nil {
		return nil, err
	}
	a := &app{
		renderers:    [2]*untiemygo.Renderer{texRenderer, typstRenderer},
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
		ui.Box(c).Grow(1).Fill().Clip().Children(func() {
			if !a.ready {
				ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
					drawMessage(p, r, a.message, t.Danger)
				})
				return
			}
			a.renderer().Element(c, *a.source(), untie.Display).Fill()
		})
	})
}

func (a *app) renderer() *untiemygo.Renderer {
	return a.renderers[a.dialect]
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
	result, err := a.renderer().Render(
		context.Background(),
		*a.source(),
		untie.Display,
		untie.Style{},
	)
	a.diagnostics = nil
	a.ready = false
	if result != nil {
		a.diagnostics = result.Diagnostics
		a.ready = result.Box != nil
	}
	a.message = diagnosticMessage(a.diagnostics)
	if err != nil {
		a.message = layoutErrorMessage(err)
	}
}

func layoutErrorMessage(err error) string {
	var layoutErr *untie.LayoutError
	if !errors.As(err, &layoutErr) {
		return err.Error()
	}
	if layoutErr.Range.IsValid() && !layoutErr.Range.IsDetached() {
		return fmt.Sprintf("%s at bytes [%d,%d): %v",
			layoutErr.Kind, layoutErr.Range.Start, layoutErr.Range.End, layoutErr.Err)
	}
	return fmt.Sprintf("%s: %v", layoutErr.Kind, layoutErr.Err)
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
