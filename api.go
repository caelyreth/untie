package untie

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/engine"
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
	Style      = display.Style
	FontData   = font.FontData
	Fonts      = font.Fonts
	FontSet    = font.FontSet
)

const (
	SeverityError   = source.SeverityError
	SeverityWarning = source.SeverityWarning
	SeverityInfo    = source.SeverityInfo
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
	// ErrInvalidContext reports a nil context.
	ErrInvalidContext = errors.New("untie: nil context")
	// ErrInvalidExpr reports a nil expression.
	ErrInvalidExpr = errors.New("untie: nil expression")
	// ErrInvalidFonts reports a missing prepared font set.
	ErrInvalidFonts = errors.New("untie: nil font set")
	// ErrInvalidStyle reports an invalid host layout style.
	ErrInvalidStyle = errors.New("untie: invalid layout style")
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

// LayoutContext lays out an expression with cancellation and resource limits.
//
// The facade currently constructs an engine per call. A renderer registry keyed
// by FontSet and layout limits is a follow-up optimization, not part of this
// correctness boundary.
func LayoutContext(ctx context.Context, e *Expr, fonts *FontSet, style Style) (*Box, error) {
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, ErrInvalidExpr
	}
	if fonts == nil {
		return nil, ErrInvalidFonts
	}
	if err := validateLayoutStyle(style); err != nil {
		return nil, err
	}
	root := e.rootForLayout()
	if root == nil || root.Root == nil {
		return nil, ErrInvalidExpr
	}
	layoutEngine, err := engine.New(fonts, e.limits)
	if err != nil {
		return nil, fmt.Errorf("untie: prepare layout: %w", err)
	}
	box, err := layoutEngine.Layout(ctx, root.Root, e.mode, style)
	if err != nil {
		return nil, err
	}
	if box == nil {
		return nil, fmt.Errorf("untie: layout returned a nil box")
	}
	if err := box.Validate(); err != nil {
		return nil, fmt.Errorf("untie: invalid layout output: %w", err)
	}
	return box, nil
}

// Layout lays out an expression with a background context.
func Layout(e *Expr, fonts *FontSet, style Style) *Box {
	box, err := LayoutContext(context.Background(), e, fonts, style)
	if err != nil {
		return nil
	}
	return box
}

func validateLayoutStyle(style Style) error {
	if style.Size <= 0 || math.IsNaN(float64(style.Size)) || math.IsInf(float64(style.Size), 0) {
		return fmt.Errorf("%w: size must be finite and positive", ErrInvalidStyle)
	}
	if style.MaxWidth < 0 || math.IsNaN(float64(style.MaxWidth)) || math.IsInf(float64(style.MaxWidth), 0) {
		return fmt.Errorf("%w: max width must be finite and non-negative", ErrInvalidStyle)
	}
	if style.Align > display.AlignEnd {
		return fmt.Errorf("%w: unknown alignment %d", ErrInvalidStyle, style.Align)
	}
	if style.ScriptLevel < 0 {
		return fmt.Errorf("%w: negative script level", ErrInvalidStyle)
	}
	return nil
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
