package mygo_test

import (
	"fmt"
	"os"

	"github.com/caelyreth/untie"
	adapter "github.com/caelyreth/untie/contrib/mygo"
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
	math, err := adapter.New(fonts, adapter.Options{CacheEntries: 8})
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
