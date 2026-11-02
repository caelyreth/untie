// Command formula-check parses one formula and prints its diagnostics and
// canonical IR. It is a development and fixture-inspection tool.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/ir"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("formula-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dialectName := flags.String("dialect", "tex", "tex or typst")
	display := flags.Bool("display", false, "parse in display mode")
	sourceFile := flags.String("f", "", "source file; reads stdin when empty")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: formula-check [-dialect tex|typst] [-display] [-f FILE] [SOURCE]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}

	dialect, err := parseDialect(*dialectName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	src, err := readSource(*sourceFile, flags.Args(), stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	mode := untie.Inline
	if *display {
		mode = untie.Display
	}
	expr := untie.Parse(src, dialect, mode, untie.ParseOptions{})
	for _, diagnostic := range expr.Diagnostics() {
		fmt.Fprintf(stdout, "%d:%d: %s: %s\n",
			diagnostic.Range.Start, diagnostic.Range.End, diagnostic.Code, diagnostic.Message)
	}
	canonical, err := ir.MarshalCanonical(expr.Root())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, string(canonical))
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

func readSource(file string, args []string, stdin io.Reader) (string, error) {
	if file != "" {
		if len(args) != 0 {
			return "", fmt.Errorf("-f and SOURCE cannot both be used")
		}
		data, err := os.ReadFile(file)
		return string(data), err
	}
	if len(args) > 1 {
		return "", fmt.Errorf("expected at most one SOURCE argument")
	}
	if len(args) == 1 {
		return args[0], nil
	}
	data, err := io.ReadAll(stdin)
	return string(data), err
}
