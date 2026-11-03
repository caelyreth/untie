// Command svg renders one formula as a standalone SVG document.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/encode/svg"
	"github.com/caelyreth/untie/fontsource"
)

const defaultFontPath = "testdata/upstream/fonts/NewCMMath-Book.otf"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("svg", flag.ContinueOnError)
	fontPath := flags.String("font", defaultFontPath, "OpenType MATH font path")
	source := flags.String("source", `\frac{-b \pm \sqrt{b^2-4ac}}{2a}`, "formula source")
	dialectName := flags.String("dialect", "tex", "input dialect: tex or typst")
	modeName := flags.String("mode", "display", "layout mode: display or inline")
	size := flags.Float64("size", 32, "layout size in host units")
	padding := flags.Float64("padding", 12, "SVG padding in host units")
	precision := flags.Int("precision", 3, "maximum fractional digits")
	output := flags.String("out", "", "output path; empty writes to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}

	dialect, err := parseDialect(*dialectName)
	if err != nil {
		return err
	}
	mode, err := parseMode(*modeName)
	if err != nil {
		return err
	}
	box, err := renderBox(*fontPath, *source, dialect, mode, float32(*size))
	if err != nil {
		return err
	}

	options := svg.DefaultOptions()
	options.Padding = float32(*padding)
	options.Precision = *precision
	options.Description = *source
	if *output == "" {
		return svg.Write(stdout, box, &options)
	}

	file, err := os.Create(*output)
	if err != nil {
		return fmt.Errorf("create %s: %w", *output, err)
	}
	if err := svg.Write(file, box, &options); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", *output, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", *output, err)
	}
	return nil
}

func renderBox(fontPath, source string, dialect untie.Dialect, mode untie.Mode, size float32) (*untie.Box, error) {
	fonts, err := fontsource.LoadMath(
		os.DirFS(filepath.Dir(fontPath)),
		filepath.Base(fontPath),
		untie.WithFontID("example-math"),
	)
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
		DefaultDialect: dialect,
		DefaultStyle:   untie.Style{Size: size},
		CacheEntries:   8,
	})
	if err != nil {
		return nil, fmt.Errorf("create renderer: %w", err)
	}
	result, err := renderer.Render(context.Background(), source, untie.RenderRequest{Mode: mode})
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
