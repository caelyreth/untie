// Command mathml converts TeX or Typst math to deterministic Presentation MathML.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/mathml"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("mathml", flag.ContinueOnError)
	source := flags.String("source", `x^2+\frac{1}{2}`, "formula source")
	dialectName := flags.String("dialect", "tex", "tex or typst")
	modeName := flags.String("mode", "display", "inline or display")
	output := flags.String("out", "", "output path; empty writes to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}

	var dialect untie.Dialect
	switch *dialectName {
	case "tex":
		dialect = untie.TeX
	case "typst":
		dialect = untie.Typst
	default:
		return fmt.Errorf("unknown dialect %q", *dialectName)
	}
	var mode untie.Mode
	switch *modeName {
	case "inline":
		mode = untie.Inline
	case "display":
		mode = untie.Display
	default:
		return fmt.Errorf("unknown mode %q", *modeName)
	}

	expr := untie.Parse(*source, dialect, mode, untie.ParseOptions{})
	formula, err := expr.RootSnapshot()
	if err != nil {
		return err
	}
	document, err := mathml.Marshal(context.Background(), formula, mode, mathml.Options{})
	if err != nil {
		return err
	}
	if *output == "" {
		_, err = stdout.Write(document)
		return err
	}
	if err := os.WriteFile(*output, document, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *output, err)
	}
	return nil
}
