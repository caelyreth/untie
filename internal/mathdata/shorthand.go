package mathdata

import "strings"

// TypstShorthand is one Typst math-mode source shorthand.
type TypstShorthand struct {
	Syntax string
	Value  string
}

// LookupTypstShorthand returns the symbol value for an exact shorthand.
func LookupTypstShorthand(syntax string) (TypstShorthand, bool) {
	return lookupTypstShorthand(syntax)
}

// MatchTypstShorthandPrefix returns the longest shorthand matching the start of
// text.
//
// The table is small and deliberately kept separate from lexical precedence.
// Callers decide whether the returned candidate is legal at their lexer state.
func MatchTypstShorthandPrefix(text string) (TypstShorthand, bool) {
	var best TypstShorthand
	found := false
	for _, shorthand := range typstShorthands {
		if len(shorthand.Syntax) <= len(best.Syntax) ||
			!strings.HasPrefix(text, shorthand.Syntax) {
			continue
		}
		best = shorthand
		found = true
	}
	return best, found
}
