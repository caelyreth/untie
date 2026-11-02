package untie_test

import (
	"fmt"

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
