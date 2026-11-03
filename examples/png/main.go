// Command png renders one formula as an antialiased PNG image.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"

	"github.com/caelyreth/untie/encode/raster"
	"github.com/caelyreth/untie/examples/internal/example"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("png", flag.ContinueOnError)
	renderConfig := example.BindRenderFlags(flags)
	scale := flags.Float64("scale", 2, "output pixels per host unit")
	padding := flags.Float64("padding", 12, "padding in output pixels")
	transparent := flags.Bool("transparent", false, "leave the background transparent")
	output := flags.String("out", "", "output path; empty writes to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}

	box, err := renderConfig.Render(context.Background())
	if err != nil {
		return err
	}
	options := raster.DefaultOptions()
	options.Scale = float32(*scale)
	options.Padding = float32(*padding)
	if !*transparent {
		options.Background = &color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	if *output == "" {
		return raster.EncodePNG(stdout, box, &options)
	}

	file, err := os.Create(*output)
	if err != nil {
		return fmt.Errorf("create %s: %w", *output, err)
	}
	if err := raster.EncodePNG(file, box, &options); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", *output, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", *output, err)
	}
	return nil
}
