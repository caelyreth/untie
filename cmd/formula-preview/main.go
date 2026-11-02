// Command formula-preview renders one formula to SVG through the public API.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/internal/testrender"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("formula-preview", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dialectName := flags.String("dialect", "tex", "tex or typst")
	displayMode := flags.Bool("display", false, "display mode")
	fontPath := flags.String("font", "", "OpenType MATH font path")
	size := flags.Float64("size", 24, "font size in DIP")
	maxWidth := flags.Float64("max-width", 0, "maximum display width in DIP")
	output := flags.String("o", "", "output SVG file; stdout when empty")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: formula-preview -font FONT [-dialect tex|typst] [-display] [-size DIP] [SOURCE]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *fontPath == "" {
		_, _ = fmt.Fprintln(stderr, "formula-preview: -font is required")
		return 2
	}
	dialect, err := parseDialect(*dialectName)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	src, err := readSource(flags.Args(), stdin)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	fontBytes, err := os.ReadFile(*fontPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "formula-preview: read font: %v\n", err)
		return 2
	}
	fonts, err := untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: *fontPath},
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "formula-preview: load font: %v\n", err)
		return 2
	}
	mode := untie.Inline
	if *displayMode {
		mode = untie.Display
	}
	expr := untie.Parse(src, dialect, mode, untie.ParseOptions{})
	box, err := untie.LayoutContext(context.Background(), expr, fonts, untie.Style{
		Size:     float32(*size),
		MaxWidth: float32(*maxWidth),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "formula-preview: layout: %v\n", err)
		return 1
	}
	svg := testrender.SVG(box, testrender.Options{
		Padding: float32(*size) * 0.25,
		Title:   src,
	})
	if *output == "" {
		if _, err := io.WriteString(stdout, svg); err != nil {
			_, _ = fmt.Fprintf(stderr, "formula-preview: write output: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*output, []byte(svg), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "formula-preview: write %s: %v\n", *output, err)
		return 1
	}
	return 0
}

func parseDialect(value string) (untie.Dialect, error) {
	switch strings.ToLower(value) {
	case "tex", "latex", "katex":
		return untie.TeX, nil
	case "typst", "typ":
		return untie.Typst, nil
	default:
		return 0, fmt.Errorf("unknown dialect %q", value)
	}
}

func readSource(args []string, stdin io.Reader) (string, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("expected at most one SOURCE argument")
	}
	if len(args) == 1 {
		return args[0], nil
	}
	data, err := io.ReadAll(stdin)
	return string(data), err
}
