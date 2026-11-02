package typst

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type signature struct {
	maxPositional int
	named         map[string]bool
}

func signatureFor(name string) (signature, bool) {
	if sig, ok := functionSignatures[name]; ok {
		return sig, true
	}
	if _, ok := accentAliases[name]; ok {
		return signature{maxPositional: 1, named: setOf("base", "size", "dotless")}, true
	}
	if def, ok := symbol(name); ok &&
		(def.class == ir.ClassOpening || def.class == ir.ClassFence) {
		return signature{maxPositional: 1, named: setOf("body")}, true
	}
	return signature{}, false
}

var functionSignatures = map[string]signature{
	"sqrt":   {maxPositional: 1, named: setOf("radicand")},
	"root":   {maxPositional: 2, named: setOf("index", "radicand")},
	"frac":   {maxPositional: 2, named: setOf("num", "denom", "style")},
	"binom":  {maxPositional: -1, named: setOf("upper", "lower")},
	"attach": {maxPositional: 7, named: setOf("base", "t", "b", "tl", "bl", "tr", "br")},
	"scripts": {
		maxPositional: 1, named: setOf("body"),
	},
	"limits":   {maxPositional: 1, named: setOf("body", "inline")},
	"lr":       {maxPositional: 1, named: setOf("body", "size")},
	"mid":      {maxPositional: 1, named: setOf("body")},
	"abs":      {maxPositional: 1, named: setOf("body", "size")},
	"norm":     {maxPositional: 1, named: setOf("body", "size")},
	"floor":    {maxPositional: 1, named: setOf("body", "size")},
	"ceil":     {maxPositional: 1, named: setOf("body", "size")},
	"round":    {maxPositional: 1, named: setOf("body", "size")},
	"mat":      {maxPositional: -1, named: setOf("delim", "align", "augment", "gap", "row-gap", "column-gap")},
	"vec":      {maxPositional: -1, named: setOf("delim", "align", "gap")},
	"cases":    {maxPositional: -1, named: setOf("delim", "reverse", "gap")},
	"accent":   {maxPositional: 2, named: setOf("base", "accent", "size", "dotless")},
	"overline": {maxPositional: 1, named: setOf("body")},
	"underline": {
		maxPositional: 1, named: setOf("body"),
	},
	"cancel": {maxPositional: 1, named: setOf("body", "length", "inverted", "cross", "angle", "stroke")},
	"h":      {maxPositional: 1, named: setOf("amount", "weak")},
	"text": {maxPositional: -1, named: setOf(
		"font", "fallback", "style", "weight", "stretch", "size", "fill", "stroke",
		"tracking", "spacing", "cjk-latin-spacing", "baseline", "overhang", "top-edge",
		"bottom-edge", "lang", "region", "script", "dir", "hyphenate", "costs",
		"kerning", "alternates", "stylistic-set", "ligatures", "discretionary-ligatures",
		"historical-ligatures", "number-type", "number-width", "slashed-zero",
		"fractions", "features", "variations", "body", "text",
	)},
	"strong": {maxPositional: 1, named: setOf("body", "delta")},
	"emph":   {maxPositional: 1, named: setOf("body")},
	"link":   {maxPositional: 2, named: setOf("dest", "body")},
	"box": {maxPositional: 1, named: setOf(
		"width", "height", "baseline", "fill", "stroke", "border", "radius",
		"inset", "outset", "clip", "body",
	)},
	"hide":    {maxPositional: 1, named: setOf("body")},
	"display": {maxPositional: 1, named: setOf("body", "cramped")},
	"inline":  {maxPositional: 1, named: setOf("body", "cramped")},
	"script":  {maxPositional: 1, named: setOf("body", "cramped")},
	"sscript": {maxPositional: 1, named: setOf("body", "cramped")},
	"bold":    {maxPositional: 1, named: setOf("body")},
	"italic":  {maxPositional: 1, named: setOf("body")},
	"upright": {maxPositional: 1, named: setOf("body")},
	"serif":   {maxPositional: 1, named: setOf("body")},
	"sans":    {maxPositional: 1, named: setOf("body")},
	"mono":    {maxPositional: 1, named: setOf("body")},
	"bb":      {maxPositional: 1, named: setOf("body")},
	"cal":     {maxPositional: 1, named: setOf("body")},
	"scr":     {maxPositional: 1, named: setOf("body")},
	"frak":    {maxPositional: 1, named: setOf("body")},
	"class":   {maxPositional: 2, named: setOf("class", "body")},
	"op":      {maxPositional: 1, named: setOf("text", "limits")},
	"stretch": {maxPositional: 1, named: setOf("body", "size")},
	"primes":  {maxPositional: 1, named: setOf("count")},
	"overbrace": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"underbrace": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"overbracket": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"underbracket": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"overparen": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"underparen": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"overshell": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"undershell": {
		maxPositional: 2, named: setOf("body", "annotation"),
	},
	"equation": {maxPositional: -1, named: setOf(
		"block", "numbering", "number-align", "supplement", "alt", "body",
	)},
	"rgb":  {maxPositional: 4, named: setOf("red", "green", "blue", "alpha", "hex")},
	"luma": {maxPositional: 2, named: setOf("lightness", "alpha")},
}

func setOf(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func (l *lowerer) validateSignature(call *astNode, name string) {
	sig, ok := signatureFor(name)
	if !ok {
		return
	}
	if sig.maxPositional >= 0 {
		positional := l.positionalArguments(call)
		if len(positional) > sig.maxPositional {
			for _, arg := range positional[sig.maxPositional:] {
				l.errorAt(arg.span, source.CodeSyntax, "unexpected argument")
			}
		}
	}
	for _, arg := range call.args {
		if arg.kind == argNamed && !sig.named[arg.name] {
			l.errorAt(arg.span, source.CodeSyntax, "unexpected argument: %s", arg.name)
		}
	}
}
