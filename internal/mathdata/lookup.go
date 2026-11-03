package mathdata

import "sort"

func lookupUnicodeClass(r rune) (Class, bool) {
	i := sort.Search(len(unicodeClassRanges), func(i int) bool {
		return unicodeClassRanges[i].end > r
	})
	if i < len(unicodeClassRanges) && unicodeClassRanges[i].start <= r {
		return unicodeClassRanges[i].class, true
	}
	return ClassNormal, false
}

func lookupTeXSymbol(name string) (TeXSymbol, bool) {
	i := sort.Search(len(texSymbols), func(i int) bool {
		return texSymbols[i].Command >= name
	})
	if i < len(texSymbols) && texSymbols[i].Command == name {
		return texSymbols[i], true
	}
	return TeXSymbol{}, false
}

func lookupTypstSymbol(path string) (TypstSymbol, bool) {
	i := sort.Search(len(typstSymbols), func(i int) bool {
		return typstSymbols[i].Path >= path
	})
	if i < len(typstSymbols) && typstSymbols[i].Path == path {
		return typstSymbols[i], true
	}
	return TypstSymbol{}, false
}

func lookupNegatedRelation(base string) (string, bool) {
	i := sort.Search(len(negatedRelations), func(i int) bool {
		return negatedRelations[i].base >= base
	})
	if i < len(negatedRelations) && negatedRelations[i].base == base {
		return negatedRelations[i].negated, true
	}
	return "", false
}

func lookupTypstShorthand(text string) (TypstShorthand, bool) {
	i := sort.Search(len(typstShorthands), func(i int) bool {
		return typstShorthands[i].Syntax >= text
	})
	if i < len(typstShorthands) && typstShorthands[i].Syntax == text {
		return typstShorthands[i], true
	}
	return TypstShorthand{}, false
}

func lookupTeXOperator(name string) (Operator, bool) {
	i := sort.Search(len(texOperators), func(i int) bool {
		return texOperators[i].Name >= name
	})
	if i < len(texOperators) && texOperators[i].Name == name {
		return texOperators[i], true
	}
	return Operator{}, false
}

func lookupTypstOperator(name string) (Operator, bool) {
	i := sort.Search(len(typstOperators), func(i int) bool {
		return typstOperators[i].Name >= name
	})
	if i < len(typstOperators) && typstOperators[i].Name == name {
		return typstOperators[i], true
	}
	return Operator{}, false
}
