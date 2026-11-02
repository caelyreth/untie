package untie

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/internal/typst"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// Dialect selects the input math language.
type Dialect uint8

const (
	TeX Dialect = iota
	Typst
)

// Mode selects root math style.
type Mode = ir.Mode

const (
	Inline  = ir.Inline
	Display = ir.Display
)

// Limits, diagnostics, and output types are aliases of their owning packages.
type (
	Limits     = source.Limits
	Range      = source.Range
	Diagnostic = source.Diagnostic
	Severity   = source.Severity
	Box        = display.Box
	Item       = display.Item
	ItemKind   = display.ItemKind
	Paint      = display.Paint
	Role       = display.Role
	RGBA       = display.RGBA
	Hit        = display.Hit
	HitKind    = display.HitKind
	Break      = display.Break
	BreakKind  = display.BreakKind
	Align      = display.Align
	FontData   = font.FontData
	Fonts      = font.Fonts
	FontSet    = font.FontSet
)

// Style is a display.Style layout request. Style.Align is currently inactive:
// it is validated for forward compatibility but does not change output geometry.
type Style = display.Style

const (
	SeverityError   = source.SeverityError
	SeverityWarning = source.SeverityWarning
	SeverityInfo    = source.SeverityInfo

	ItemGlyphPath = display.ItemGlyphPath
	ItemRule      = display.ItemRule
	ItemGlyphRef  = display.ItemGlyphRef

	RoleInk         = display.RoleInk
	RoleError       = display.RoleError
	RolePlaceholder = display.RolePlaceholder

	HitAtom                = display.HitAtom
	HitGroup               = display.HitGroup
	HitScript              = display.HitScript
	HitFractionNumerator   = display.HitFractionNumerator
	HitFractionDenominator = display.HitFractionDenominator
	HitRadical             = display.HitRadical
	HitFence               = display.HitFence
	HitTableCell           = display.HitTableCell
	HitRow                 = display.HitRow
	HitOther               = display.HitOther

	BreakRelation = display.BreakRelation
	BreakBinary   = display.BreakBinary
	BreakExplicit = display.BreakExplicit

	AlignStart  = display.AlignStart
	AlignCenter = display.AlignCenter
	AlignEnd    = display.AlignEnd
)

// Macro is a host-provided TeX macro.
type Macro struct {
	Body string
	Args int
}

// ParseOptions configure parsing. TeX macros and the Typst preamble are
// dialect-specific and ignored by the other frontend.
type ParseOptions struct {
	Macros   map[string]Macro
	Preamble string
	Limits   Limits
}

// Expr is an immutable parsed formula.
type Expr struct {
	source      string
	dialect     Dialect
	mode        Mode
	root        *ir.Formula
	limits      source.Limits
	diagnostics []source.Diagnostic
	key         uint64
}

var (
	// ErrInvalidExpr reports a nil expression.
	ErrInvalidExpr = errors.New("untie: nil expression")
)

// Parse parses source into an expression. Malformed source produces diagnostics
// and a drawable fallback tree rather than an ordinary error.
func Parse(src string, dialect Dialect, mode Mode, opts ParseOptions) (expr *Expr) {
	limits := opts.Limits.WithDefaults()
	return parseRecovered(src, dialect, mode, opts, limits, func(limits source.Limits) frontendResult {
		switch dialect {
		case TeX:
			macros := make(map[string]tex.Macro, len(opts.Macros))
			for name, macro := range opts.Macros {
				macros[name] = tex.Macro{Body: macro.Body, Args: macro.Args}
			}
			result := tex.Parse(src, mode, tex.Options{Macros: macros, Limits: limits})
			return frontendResult{root: result.Root, diagnostics: result.Diagnostics}
		case Typst:
			result := typst.Parse(src, mode, typst.Options{Preamble: opts.Preamble, Limits: limits})
			return frontendResult{root: result.Root, diagnostics: result.Diagnostics}
		default:
			span := source.NewRange(0, len(src))
			return frontendResult{
				root: fallbackRoot(src, limits),
				diagnostics: []source.Diagnostic{{
					Range:    span,
					Severity: source.SeverityError,
					Code:     source.CodeInvalidValue,
					Message:  "unknown math dialect",
				}},
			}
		}
	})
}

type frontendResult struct {
	root        ir.Node
	diagnostics []source.Diagnostic
}

type frontendParser func(source.Limits) frontendResult

func parseRecovered(
	src string,
	dialect Dialect,
	mode Mode,
	opts ParseOptions,
	limits source.Limits,
	parser frontendParser,
) (expr *Expr) {
	root := fallbackFormula(src, limits)
	var diagnostics []source.Diagnostic
	defer func() {
		if recovered := recover(); recovered != nil {
			diagnostic := source.Diagnostic{
				Range:    source.NewRange(0, len(src)),
				Severity: source.SeverityError,
				Code:     source.CodeInternal,
				Message:  fmt.Sprintf("parser invariant failure: %v", recovered),
			}
			root = fallbackFormula(src, limits)
			diagnostics = []source.Diagnostic{diagnostic}
			expr = newExpr(src, dialect, mode, limits, root, diagnostics, opts)
		}
	}()

	if len(src) > limits.MaxSourceBytes {
		diagnostics = append(diagnostics, source.Diagnostic{
			Range:    source.NewRange(0, len(src)),
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "source exceeds configured byte limit",
		})
	} else {
		result := parser(limits)
		candidate := &ir.Formula{Root: result.root}
		if result.root == nil {
			diagnostics = append(diagnostics, result.diagnostics...)
			diagnostics = append(diagnostics, source.Diagnostic{
				Range:    source.NewRange(0, len(src)),
				Severity: source.SeverityError,
				Code:     source.CodeInternal,
				Message:  "frontend returned a nil IR root",
			})
		} else if err := ir.ValidateSource(candidate, src, limits); err != nil {
			diagnostics = append(diagnostics, result.diagnostics...)
			diagnostics = append(diagnostics, source.Diagnostic{
				Range:    source.NewRange(0, len(src)),
				Severity: source.SeverityError,
				Code:     source.CodeInternal,
				Message:  fmt.Sprintf("frontend returned invalid IR: %v", err),
			})
		} else {
			root = candidate
			diagnostics = append(diagnostics, result.diagnostics...)
		}
	}

	return newExpr(src, dialect, mode, limits, root, diagnostics, opts)
}

func newExpr(
	src string,
	dialect Dialect,
	mode Mode,
	limits source.Limits,
	root *ir.Formula,
	diagnostics []source.Diagnostic,
	opts ParseOptions,
) *Expr {
	expr := &Expr{
		source:      src,
		dialect:     dialect,
		mode:        mode,
		root:        root,
		limits:      limits,
		diagnostics: source.NormalizeDiagnosticsFor(diagnostics, len(src)),
	}
	expr.key = computeKey(expr, limits, opts)
	return expr
}

// ParseTeX is a dialect-specific convenience wrapper.
func ParseTeX(src string, mode Mode, opts ParseOptions) *Expr {
	return Parse(src, TeX, mode, opts)
}

// ParseTypst is a dialect-specific convenience wrapper.
func ParseTypst(src string, mode Mode, opts ParseOptions) *Expr {
	return Parse(src, Typst, mode, opts)
}

// Source returns the original source.
func (e *Expr) Source() string {
	if e == nil {
		return ""
	}
	return e.source
}

// Dialect returns the source dialect.
func (e *Expr) Dialect() Dialect {
	if e == nil {
		return TeX
	}
	return e.dialect
}

// Mode returns the root math mode.
func (e *Expr) Mode() Mode {
	if e == nil {
		return Inline
	}
	return e.mode
}

// Root returns an independent snapshot of the canonical IR. Mutating the
// snapshot never changes the expression or its Key.
func (e *Expr) Root() *ir.Formula {
	root, err := e.RootSnapshot()
	if err != nil {
		return nil
	}
	return root
}

// RootSnapshot returns an independent, validated copy of the canonical IR.
func (e *Expr) RootSnapshot() (*ir.Formula, error) {
	if e == nil {
		return nil, ErrInvalidExpr
	}
	return ir.CloneFormula(e.root, e.limits)
}

// rootForLayout exposes the owned tree to the in-package layout orchestration.
// The tree must be treated as immutable for the duration of layout.
func (e *Expr) rootForLayout() *ir.Formula {
	if e == nil {
		return nil
	}
	return e.root
}

// Diagnostics returns an independent copy of parse diagnostics.
func (e *Expr) Diagnostics() []Diagnostic {
	if e == nil {
		return nil
	}
	out := make([]Diagnostic, len(e.diagnostics))
	for i, d := range e.diagnostics {
		out[i] = d.Clone()
	}
	return out
}

// Key returns a stable convenience cache key for the parsed expression.
func (e *Expr) Key() uint64 {
	if e == nil {
		return 0
	}
	return e.key
}

// LoadFonts prepares host-supplied font bytes for repeated layout.
func LoadFonts(fonts Fonts) (*FontSet, error) {
	return font.Load(fonts)
}

func fallbackFormula(src string, limits source.Limits) *ir.Formula {
	return &ir.Formula{Root: fallbackRoot(src, limits)}
}

func fallbackRoot(src string, limits source.Limits) ir.Node {
	span := source.NewRange(0, len(src))
	row := &ir.Row{Base: ir.NewBase(ir.KindRow, span)}
	if src != "" {
		row.Items = []ir.Node{ir.NewError(span, truncateUTF8(src, limits.MaxTextBytes), source.CodeUnsupported)}
	}
	return row
}

func truncateUTF8(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	text = text[:limit]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

func computeKey(e *Expr, limits source.Limits, opts ParseOptions) uint64 {
	h := sha256.New()
	writeString := func(s string) {
		var size [8]byte
		binary.LittleEndian.PutUint64(size[:], uint64(len(s)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(s))
	}
	writeInt := func(v int64) {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		_, _ = h.Write(buf[:])
	}

	writeString(Version())
	writeInt(int64(e.dialect))
	writeInt(int64(e.mode))
	writeString(e.source)
	writeString(opts.Preamble)
	for _, limit := range []int{
		limits.MaxSourceBytes,
		limits.MaxTokens,
		limits.MaxNesting,
		limits.MaxNodes,
		limits.MaxMacroExpansions,
		limits.MaxExpandedTokens,
		limits.MaxCallDepth,
		limits.MaxTableCells,
		limits.MaxAssemblyRepeats,
		limits.MaxTextBytes,
		limits.MaxOutputItems,
		limits.MaxPathSegments,
	} {
		writeInt(int64(limit))
	}
	writeInt(limits.MaxDimension)

	names := make([]string, 0, len(opts.Macros))
	for name := range opts.Macros {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		macro := opts.Macros[name]
		writeString(name)
		writeString(macro.Body)
		writeInt(int64(macro.Args))
	}
	sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(sum[:8])
}
