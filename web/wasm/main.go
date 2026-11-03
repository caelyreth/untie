package main

import (
	"context"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/encode/svg"
)

var state struct {
	mu       sync.Mutex
	renderer *untie.Renderer
}

func main() {
	js.Global().Set("untieAPI", js.ValueOf(map[string]any{
		"initFont":  js.FuncOf(initFont),
		"renderSVG": js.FuncOf(renderSVG),
	}))
	select {}
}

func initFont(_ js.Value, args []js.Value) any {
	if len(args) != 1 || args[0].Type() != js.TypeObject {
		return "initFont expects a Uint8Array"
	}
	data := make([]byte, args[0].Length())
	js.CopyBytesToGo(data, args[0])
	fonts, err := untie.LoadMathFont(data, untie.WithFontID("untie-web"))
	if err != nil {
		return fmt.Sprintf("load font: %v", err)
	}
	renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
		DefaultStyle: untie.Style{Size: 32},
		CacheEntries: 32,
	})
	if err != nil {
		return fmt.Sprintf("create renderer: %v", err)
	}
	state.mu.Lock()
	state.renderer = renderer
	state.mu.Unlock()
	return ""
}

func renderSVG(_ js.Value, args []js.Value) any {
	if len(args) != 4 {
		return renderError("renderSVG expects source, dialect, display, and size")
	}
	source := args[0].String()
	dialect, err := parseDialect(args[1].String())
	if err != nil {
		return renderError(err.Error())
	}
	mode := untie.Inline
	if args[2].Bool() {
		mode = untie.Display
	}
	size := float32(args[3].Float())
	if size <= 0 {
		size = 32
	}

	state.mu.Lock()
	renderer := state.renderer
	state.mu.Unlock()
	if renderer == nil {
		return renderError("font has not been initialized")
	}
	result, err := renderer.Render(context.Background(), source, untie.RenderRequest{
		Dialect: dialect,
		Mode:    mode,
		Style:   untie.Style{Size: size},
	})
	if err != nil {
		return renderError(err.Error())
	}
	document, err := svg.Marshal(result.Box, nil)
	if err != nil {
		return renderError(err.Error())
	}
	return map[string]any{"svg": string(document), "error": ""}
}

func parseDialect(name string) (untie.Dialect, error) {
	switch name {
	case "typst":
		return untie.Typst, nil
	case "", "tex":
		return untie.TeX, nil
	default:
		return 0, fmt.Errorf("unknown dialect %q", name)
	}
}

func renderError(message string) map[string]any {
	return map[string]any{"svg": "", "error": message}
}
