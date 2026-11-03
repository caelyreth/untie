package untiemygo_test

import (
	"context"
	"fmt"
	"os"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/contrib/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func Example() {
	data, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		panic(err)
	}
	fonts, err := untie.LoadMathFont(data)
	if err != nil {
		panic(err)
	}
	math, err := untiemygo.New(fonts, untiemygo.Options{CacheEntries: 8})
	if err != nil {
		panic(err)
	}

	var options markdown.Options
	math.BindMarkdown(&options)
	ui.NewTester(func(c *ui.Context) {
		markdown.View(c, `Euler: $e^{i\pi}+1=0$.`, &options)
	}, 400, 120)

	fmt.Println(math.CacheStats().Entries)
	// Output: 1
}

func ExampleRenderer_Element() {
	data, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		panic(err)
	}
	fonts, err := untie.LoadMathFont(data)
	if err != nil {
		panic(err)
	}
	math, err := untiemygo.New(fonts, untiemygo.Options{CacheEntries: 8})
	if err != nil {
		panic(err)
	}

	ui.NewTester(func(c *ui.Context) {
		math.Element(c, `e^{i\pi}+1=0`, untie.Display).FillWidth()
	}, 400, 120)

	fmt.Println(math.CacheStats().Entries)
	// Output: 1
}

func ExampleRenderer_DisplayMathWidth() {
	data, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		panic(err)
	}
	fonts, err := untie.LoadMathFont(data)
	if err != nil {
		panic(err)
	}
	math, err := untiemygo.New(fonts, untiemygo.Options{
		DisplaySize:  24,
		CacheEntries: 8,
		Draw:         untiemygo.DrawOptions{Padding: 4},
	})
	if err != nil {
		panic(err)
	}

	const source = `a+b+c+d`
	expr := untie.Parse(source, untie.TeX, untie.Display, untie.ParseOptions{})
	label := untiemygo.DescribeFormula(source, expr.Alt()).Label
	tester := ui.NewTester(func(c *ui.Context) {
		math.DisplayMathWidth(c, source, 68)
	}, 68, 240)
	bounds, ok := tester.Find(label)
	if !ok {
		panic("width-aware math element not found")
	}
	result, err := math.Render(context.Background(), source, untie.Display, untie.Style{
		Size:        24,
		MaxWidth:    60,
		GenerateAlt: true,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(bounds.W, len(result.Box.Lines) > 1, math.CacheStats().Hits)
	// Output: 68 true 1
}
