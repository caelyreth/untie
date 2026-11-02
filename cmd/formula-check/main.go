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
	flagOutput := &errorWriter{writer: stderr}
	flags.SetOutput(flagOutput)
	dialectName := flags.String("dialect", "tex", "tex or typst")
	display := flags.Bool("display", false, "parse in display mode")
	sourceFile := flags.String("f", "", "source file; reads stdin when empty")
	flags.Usage = func() {
		if err := writeLine(flagOutput, "usage: formula-check [-dialect tex|typst] [-display] [-f FILE] [SOURCE]"); err != nil {
			return
		}
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if flagOutput.err != nil {
			return 1
		}
		return 2
	}
	if flagOutput.err != nil {
		return 1
	}

	dialect, err := parseDialect(*dialectName)
	if err != nil {
		if writeErr := writeLine(stderr, err.Error()); writeErr != nil {
			return 1
		}
		return 2
	}
	src, err := readSource(*sourceFile, flags.Args(), stdin)
	if err != nil {
		if writeErr := writeLine(stderr, err.Error()); writeErr != nil {
			return 1
		}
		return 2
	}
	mode := untie.Inline
	if *display {
		mode = untie.Display
	}
	expr := untie.Parse(src, dialect, mode, untie.ParseOptions{})
	for _, diagnostic := range expr.Diagnostics() {
		if err := writeFormat(stdout, "%d:%d: %s: %s\n",
			diagnostic.Range.Start, diagnostic.Range.End, diagnostic.Code, diagnostic.Message); err != nil {
			return 1
		}
	}
	canonical, err := ir.MarshalCanonical(expr.Root())
	if err != nil {
		if writeErr := writeLine(stderr, err.Error()); writeErr != nil {
			return 1
		}
		return 1
	}
	if err := writeLine(stdout, string(canonical)); err != nil {
		return 1
	}
	return 0
}

type errorWriter struct {
	writer io.Writer
	err    error
}

func (w *errorWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.writer.Write(data)
	if err != nil {
		w.err = err
	}
	return n, err
}

func writeFormat(writer io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(writer, format, args...)
	return err
}

func writeLine(writer io.Writer, text string) error {
	_, err := fmt.Fprintln(writer, text)
	return err
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
