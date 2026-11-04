// Command mygo-stream demonstrates streamed Markdown and morphing math.
package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed assets/NewCMMath-Book.otf
var mathFont []byte

func main() {
	model, err := newApp(mathFont)
	if err != nil {
		log.Fatal(err)
	}

	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "Untie Stream",
			Width:     980,
			Height:    760,
			MinWidth:  680,
			MinHeight: 560,
			StateKey:  "main",
			Content:   ui.View(model.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
