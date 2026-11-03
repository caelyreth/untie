// Command svg renders one formula as a standalone SVG document.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/caelyreth/untie/encode/svg"
	"github.com/caelyreth/untie/examples/internal/example"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("svg", flag.ContinueOnError)
	renderConfig := example.BindRenderFlags(flags)
	padding := flags.Float64("padding", 12, "SVG padding in host units")
	precision := flags.Int("precision", 3, "maximum fractional digits")
	output := flags.String("out", "", "output path; empty writes to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}

	box, err := renderConfig.Render(context.Background())
	if err != nil {
		return err
	}

	options := svg.DefaultOptions()
	options.Padding = float32(*padding)
	options.Precision = *precision
	options.Description = renderConfig.Source
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
