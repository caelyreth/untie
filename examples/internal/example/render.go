// Package example provides shared command-line setup for the exporter examples.
package example

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/fontsource"
)

const (
	// DefaultFontPath is relative to the repository root.
	DefaultFontPath = "testdata/upstream/fonts/NewCMMath-Book.otf"
	// DefaultSource exercises scripts, a fraction, and a radical.
	DefaultSource = `\frac{-b \pm \sqrt{b^2-4ac}}{2a}`
)

// RenderConfig contains the flags shared by the SVG and PNG examples.
type RenderConfig struct {
	FontPath string
	Source   string
	Dialect  string
	Mode     string
	Size     float64
}

// BindRenderFlags registers the shared render flags and returns their values.
func BindRenderFlags(flags *flag.FlagSet) *RenderConfig {
	config := &RenderConfig{}
	flags.StringVar(&config.FontPath, "font", DefaultFontPath, "OpenType MATH font path")
	flags.StringVar(&config.Source, "source", DefaultSource, "formula source")
	flags.StringVar(&config.Dialect, "dialect", "tex", "input dialect: tex or typst")
	flags.StringVar(&config.Mode, "mode", "display", "layout mode: display or inline")
	flags.Float64Var(&config.Size, "size", 32, "layout size in host units")
	return config
}

// Render parses source and returns one layout box.
func (c RenderConfig) Render(ctx context.Context) (*untie.Box, error) {
	dialect, err := parseDialect(c.Dialect)
	if err != nil {
		return nil, err
	}
	mode, err := parseMode(c.Mode)
	if err != nil {
		return nil, err
	}
	fonts, err := fontsource.LoadMath(
		os.DirFS(filepath.Dir(c.FontPath)),
		filepath.Base(c.FontPath),
		untie.WithFontID("example-math"),
	)
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
		DefaultDialect: dialect,
		DefaultStyle:   untie.Style{Size: float32(c.Size)},
		CacheEntries:   8,
	})
	if err != nil {
		return nil, fmt.Errorf("create renderer: %w", err)
	}
	result, err := renderer.Render(ctx, c.Source, untie.RenderRequest{Mode: mode})
	if err != nil {
		return nil, fmt.Errorf("render formula: %w", err)
	}
	return result.Box, nil
}

func parseDialect(name string) (untie.Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "tex":
		return untie.TeX, nil
	case "typst":
		return untie.Typst, nil
	default:
		return 0, fmt.Errorf("unknown dialect %q: use tex or typst", name)
	}
}

func parseMode(name string) (untie.Mode, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "display":
		return untie.Display, nil
	case "inline":
		return untie.Inline, nil
	default:
		return 0, fmt.Errorf("unknown mode %q: use display or inline", name)
	}
}
