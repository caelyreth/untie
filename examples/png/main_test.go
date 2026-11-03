package main

import (
	"bytes"
	"image/png"
	"path/filepath"
	"testing"
)

func TestRunWritesPNG(t *testing.T) {
	fontPath := filepath.Join("..", "..", "testdata", "upstream", "fonts", "NewCMMath-Book.otf")
	var output bytes.Buffer
	err := run([]string{
		"-font", fontPath,
		"-source", `\sqrt{x^2+y^2}`,
		"-scale", "2",
		"-padding", "10",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.Decode(&output)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if bounds := image.Bounds(); bounds.Dx() <= 20 || bounds.Dy() <= 20 {
		t.Fatalf("image bounds = %v, want space around the formula", bounds)
	}
	background := image.At(0, 0)
	painted := false
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y && !painted; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			if image.At(x, y) != background {
				painted = true
				break
			}
		}
	}
	if !painted {
		t.Fatal("PNG contains no visible formula pixels")
	}
}
