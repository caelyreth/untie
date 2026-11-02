package conformance

import "github.com/caelyreth/untie/source"

// ExplicitTeXLimits is deliberately distinct from DefaultLimits. It is large
// enough for every tracked formula while still bounding the parser much more
// tightly than an unbounded configuration.
func ExplicitTeXLimits() source.Limits {
	return source.Limits{
		MaxSourceBytes:     64 << 10,
		MaxTokens:          16_384,
		MaxNesting:         96,
		MaxNodes:           16_384,
		MaxMacroExpansions: 4_096,
		MaxExpandedTokens:  32_768,
		MaxCallDepth:       48,
		MaxTableCells:      2_048,
		MaxAssemblyRepeats: 512,
		MaxTextBytes:       16_384,
		MaxOutputItems:     32_768,
		MaxPathSegments:    262_144,
		MaxDimension:       source.DefaultLimits().MaxDimension,
	}
}

// ExplicitTypstLimits is distinct from DefaultLimits and large enough for every
// imported selected Typst case, including the parser-depth stress records.
// MaxNesting intentionally matches the default because those records cross the
// default boundary; the other tighter budgets still prove parity is not
// accidental.
func ExplicitTypstLimits() source.Limits {
	return source.Limits{
		MaxSourceBytes:     64 << 10,
		MaxTokens:          16_384,
		MaxNesting:         source.DefaultLimits().MaxNesting,
		MaxNodes:           16_384,
		MaxMacroExpansions: 1,
		MaxExpandedTokens:  32_768,
		MaxCallDepth:       48,
		MaxTableCells:      2_048,
		MaxAssemblyRepeats: 512,
		MaxTextBytes:       16_384,
		MaxOutputItems:     32_768,
		MaxPathSegments:    262_144,
		MaxDimension:       source.DefaultLimits().MaxDimension,
	}
}
