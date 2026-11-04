package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/untie"
	untiemygo "github.com/caelyreth/untie/contrib/mygo"
	"github.com/caelyreth/untie/render"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const defaultStreamSpeed = 165

type page uint8

const (
	editorPage page = iota
	renderPage
)

const initialPseudoStream = `## A formula arriving

Untie turns this pseudo model response into **Markdown with live math** while the reply is still streaming.

$$
G_{\mu\nu} + \Lambda g_{\mu\nu}
= \frac{8\pi G}{c^4} T_{\mu\nu}
$$

Inline math such as $E = mc^2$ joins the surrounding baseline, while display math can rebuild on every new token.

The TeX parser is recoverable, so unfinished commands still produce drawable output instead of a blank panel.

> Edit this source, then render the stream again.
`

type app struct {
	renderer *untiemygo.Renderer
	markdown markdown.Options

	source      string
	streamSpeed float64

	streaming bool
	complete  bool
	started   time.Time
	shown     int

	page   page
	scroll ui.ScrollState
	math   ui.ScrollState

	mathStreams []*untiemygo.StreamingMath
	displayAt   int
}

func newApp(fontBytes []byte) (*app, error) {
	if len(fontBytes) == 0 {
		return nil, errors.New("empty math font")
	}
	fonts, err := untie.LoadMathFont(fontBytes, untie.WithFontID("newcm-math"))
	if err != nil {
		return nil, err
	}
	renderer, err := untiemygo.New(fonts, untiemygo.Options{
		Dialect:      untie.TeX,
		BaseSize:     16,
		DisplaySize:  30,
		CacheEntries: 256,
		Draw: untiemygo.DrawOptions{
			Padding: 10,
			Align:   render.AlignCenter,
		},
	})
	if err != nil {
		return nil, err
	}
	a := &app{
		renderer:    renderer,
		source:      initialPseudoStream,
		streamSpeed: defaultStreamSpeed,
	}
	a.markdown.Extensions = parser.Default
	a.markdown.Reveal = &markdown.Reveal{Unit: markdown.AtOnce, Fade: -1}
	a.renderer.BindMarkdown(&a.markdown)
	a.markdown.Math = a.displayMath
	return a, nil
}

func (a *app) view(c *ui.Context) {
	if a.page == renderPage {
		a.renderPage(c)
		return
	}
	a.editorPage(c)
}

func (a *app) editorPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(24).Gap(16).Children(func() {
		ui.Row(c).Height(52).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "Streaming Markdown").FontSize(20).FontWeight(650)
				ui.Text(c, "Pseudo model output with morphing Untie math").
					FontSize(12).TextColor(t.TextMuted)
			})
			ui.Text(c, "MyGo + Untie").Grow(1).TextAlign(ui.End).
				FontSize(12).TextColor(t.Accent)
		})
		ui.Row(c).Height(28).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "Pseudo stream content").Bold()
			ui.Textf(c, "%d bytes", len(a.source)).Grow(1).
				TextAlign(ui.End).FontSize(12).TextColor(t.TextMuted)
		})
		if ui.TextArea(c, &a.source).Label("Pseudo stream content").
			Font("monospace").FontSize(13).Padding(12).Grow(1).
			FillHeight().Background(t.Background).Radius(10).
			Border(1, t.Border).Changed() {
			a.reset()
		}
		ui.Row(c).Height(52).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "Stream speed").FontSize(12).TextColor(t.TextMuted)
				ui.NumberInput(c, &a.streamSpeed, 20, 1000, 5).
					Label("Stream speed").Width(170)
			})
			ui.Text(c, "bytes / second").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "The preview uses MyGo's incremental Markdown stream.").
				FontSize(12).TextColor(t.TextMuted).Grow(1).TextAlign(ui.End)
			label := "Render stream"
			if a.complete {
				label = "Render again"
			}
			if ui.PrimaryButton(c, label).Clicked() {
				a.page = renderPage
				a.start(c.Now())
			}
		})
	})
}

func (a *app) renderPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Padding(24).Gap(14).Children(func() {
		ui.Row(c).Height(48).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "Rendered stream").FontSize(20).FontWeight(650)
				status := "ready to stream"
				if a.streaming {
					status = fmt.Sprintf("streaming %d%% at %.0f bytes / second", a.progress(), a.streamSpeed)
				} else if a.complete {
					status = fmt.Sprintf("complete at %.0f bytes / second", a.streamSpeed)
				}
				ui.Text(c, status).FontSize(12).TextColor(t.Accent)
			})
			ui.Text(c, "Markdown + TeX").Grow(1).TextAlign(ui.End).
				FontSize(12).TextColor(t.TextMuted)
			if ui.Button(c, "Edit source").Clicked() {
				a.reset()
				a.page = editorPage
			}
		})
		ui.Box(c).Height(1).Background(t.Border)
		ui.Scroll(c).TrackScroll(&a.scroll).Grow(1).MinHeight(0).
			Padding(4, 8).Children(func() {
			a.stream(c)
		})
	})
}

func (a *app) stream(c *ui.Context) {
	a.displayAt = 0
	if a.shown == 0 && !a.streaming && !a.complete {
		ui.Text(c, "Press Render stream to send the editable content through Markdown.").
			FontSize(14).TextColor(c.Theme().TextMuted)
		return
	}
	source := a.source
	if a.streaming {
		visible := int(c.Now().Sub(a.started).Seconds() * a.streamSpeed)
		visible = min(len(source), max(0, visible))
		for visible < len(source) && !utf8.RuneStart(source[visible]) {
			visible++
		}
		a.shown = visible
		source = source[:visible]
		if visible == len(a.source) {
			a.streaming = false
			a.complete = true
			c.Invalidate()
		} else {
			c.Invalidate()
		}
	}
	if a.streaming {
		markdown.Stream(c, source, &a.markdown)
	} else {
		markdown.View(c, source, &a.markdown)
	}
}

func (a *app) displayMath(c *ui.Context, source string) {
	index := a.displayAt
	a.displayAt++
	if index >= len(a.mathStreams) || a.mathStreams[index] == nil {
		ui.Text(c, source).Font("monospace").FontSize(13).TextColor(c.Theme().Danger)
		return
	}
	ui.ScrollHorizontal(c).AlignSelf(ui.Stretch).TrackScroll(&a.math).
		Margin(8, 0).Children(func() {
		a.mathStreams[index].Element(c, source, c.Now()).MinWidthPercent(100)
	})
}

func (a *app) start(now time.Time) {
	a.streaming = true
	a.complete = false
	a.started = now
	a.shown = 0
	a.scroll.Y = 0
	a.math.X, a.math.Y = 0, 0
	a.prepareDisplayMath()
}

func (a *app) reset() {
	a.streaming = false
	a.complete = false
	a.started = time.Time{}
	a.shown = 0
	a.math.X, a.math.Y = 0, 0
	a.mathStreams = a.mathStreams[:0]
	a.displayAt = 0
}

func (a *app) prepareDisplayMath() {
	a.mathStreams = a.mathStreams[:0]
	document := parser.Parse(a.source)
	for i := 0; i < document.Len(); i++ {
		if document.Block(i).Kind != parser.MathBlock {
			continue
		}
		target := strings.TrimRight(document.Lines(i), "\n")
		streamed, err := a.renderer.NewStreamingMath(
			context.Background(),
			untiemygo.StreamingMathOptions{TargetSource: target},
		)
		if err != nil {
			streamed = nil
		}
		a.mathStreams = append(a.mathStreams, streamed)
	}
	a.displayAt = 0
}

func (a *app) progress() int {
	if len(a.source) == 0 {
		return 100
	}
	return min(100, a.shown*100/len(a.source))
}
