// Command mygo renders TeX and Typst math in a native MyGo window.
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
			Title:     "Untie + MyGo",
			Width:     960,
			Height:    720,
			MinWidth:  620,
			MinHeight: 520,
			StateKey:  "main",
			Content:   ui.View(model.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
