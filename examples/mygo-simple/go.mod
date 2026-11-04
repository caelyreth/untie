module github.com/caelyreth/untie/examples/mygo-simple

go 1.27.3

tool github.com/egoist/mygo/cmd/mygo

require (
	github.com/caelyreth/untie v0.4.3
	github.com/caelyreth/untie/contrib/mygo v0.4.3
	github.com/egoist/mygo v0.7.4
)

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/go-typeset/bidi v0.3.1 // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace github.com/caelyreth/untie => ../..

replace github.com/caelyreth/untie/contrib/mygo => ../../contrib/mygo
