package untie_test

import (
	"bytes"
	"fmt"
	"os"

	"github.com/caelyreth/untie"
)

func ExampleParseTeX() {
	expr := untie.ParseTeX(`x^2+\frac{1}{2}`, untie.Display, untie.ParseOptions{})
	fmt.Println(expr.Source())
	fmt.Println(len(expr.Diagnostics()))
	// Output:
	// x^2+\frac{1}{2}
	// 0
}

func ExampleEngine_layoutAtTwoSizes() {
	fontBytes, err := os.ReadFile("testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		panic(err)
	}
	fonts, err := untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
	if err != nil {
		panic(err)
	}
	engine, err := untie.NewEngine(fonts, untie.Limits{})
	if err != nil {
		panic(err)
	}

	expr := untie.ParseTeX(`\frac{x+1}{2}`, untie.Inline, untie.ParseOptions{})
	small, err := engine.Layout(expr, untie.Style{Size: 12})
	if err != nil {
		panic(err)
	}
	before, err := small.MarshalCanonical()
	if err != nil {
		panic(err)
	}
	large, err := engine.Layout(expr, untie.Style{Size: 24})
	if err != nil {
		panic(err)
	}
	after, err := small.MarshalCanonical()
	if err != nil {
		panic(err)
	}

	fmt.Println("small valid:", small.Validate() == nil)
	fmt.Println("large valid:", large.Validate() == nil)
	fmt.Println("scales up:", large.Width > small.Width)
	fmt.Println("alt present:", small.Alt.Text != "" && small.Alt.Speech != "")
	fmt.Println("first box unchanged:", bytes.Equal(before, after))
	// Output:
	// small valid: true
	// large valid: true
	// scales up: true
	// alt present: true
	// first box unchanged: true
}

func ExampleEngine_displayLines() {
	fontBytes, err := os.ReadFile("testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		panic(err)
	}
	fonts, err := untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
	if err != nil {
		panic(err)
	}
	engine, err := untie.NewEngine(fonts, untie.Limits{})
	if err != nil {
		panic(err)
	}

	expr := untie.ParseTeX(`a+b+c+d`, untie.Display, untie.ParseOptions{})
	box, err := engine.Layout(expr, untie.Style{Size: 24, MaxWidth: 60})
	if err != nil {
		panic(err)
	}
	allValid := box.Validate() == nil
	for _, line := range box.Lines {
		allValid = allValid && line.Box.Validate() == nil
	}
	fmt.Println("wrapped:", len(box.Lines) > 1)
	fmt.Println("lines valid:", allValid)
	fmt.Println("first baseline:", displayLineBaseline(box.Lines[0]))
	// Output:
	// wrapped: true
	// lines valid: true
	// first baseline: 0
}

func displayLineBaseline(line untie.LineBox) float32 {
	return line.Baseline
}
