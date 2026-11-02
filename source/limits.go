package source

// Limits bound work for untrusted input. Zero fields are replaced with Default
// Limits by WithDefaults. Negative fields are also replaced with defaults so a
// zero-value Limits cannot accidentally become unbounded.
type Limits struct {
	MaxSourceBytes     int
	MaxTokens          int
	MaxNesting         int
	MaxNodes           int
	MaxMacroExpansions int
	MaxExpandedTokens  int
	MaxCallDepth       int
	MaxTableCells      int
	MaxAssemblyRepeats int
	MaxTextBytes       int
	MaxOutputItems     int
	MaxPathSegments    int
	MaxDimension       int64
}

// DefaultLimits returns conservative limits suitable for interactive formulas.
func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:     1 << 20,
		MaxTokens:          250_000,
		MaxNesting:         256,
		MaxNodes:           100_000,
		MaxMacroExpansions: 100_000,
		MaxExpandedTokens:  1_000_000,
		MaxCallDepth:       80,
		MaxTableCells:      10_000,
		MaxAssemblyRepeats: 1_024,
		MaxTextBytes:       100_000,
		MaxOutputItems:     250_000,
		MaxPathSegments:    2_000_000,
		MaxDimension:       1 << 40,
	}
}

// WithDefaults returns a copy where non-positive fields use defaults.
func (l Limits) WithDefaults() Limits {
	d := DefaultLimits()
	if l.MaxSourceBytes <= 0 {
		l.MaxSourceBytes = d.MaxSourceBytes
	}
	if l.MaxTokens <= 0 {
		l.MaxTokens = d.MaxTokens
	}
	if l.MaxNesting <= 0 {
		l.MaxNesting = d.MaxNesting
	}
	if l.MaxNodes <= 0 {
		l.MaxNodes = d.MaxNodes
	}
	if l.MaxMacroExpansions <= 0 {
		l.MaxMacroExpansions = d.MaxMacroExpansions
	}
	if l.MaxExpandedTokens <= 0 {
		l.MaxExpandedTokens = d.MaxExpandedTokens
	}
	if l.MaxCallDepth <= 0 {
		l.MaxCallDepth = d.MaxCallDepth
	}
	if l.MaxTableCells <= 0 {
		l.MaxTableCells = d.MaxTableCells
	}
	if l.MaxAssemblyRepeats <= 0 {
		l.MaxAssemblyRepeats = d.MaxAssemblyRepeats
	}
	if l.MaxTextBytes <= 0 {
		l.MaxTextBytes = d.MaxTextBytes
	}
	if l.MaxOutputItems <= 0 {
		l.MaxOutputItems = d.MaxOutputItems
	}
	if l.MaxPathSegments <= 0 {
		l.MaxPathSegments = d.MaxPathSegments
	}
	if l.MaxDimension <= 0 {
		l.MaxDimension = d.MaxDimension
	}
	return l
}
