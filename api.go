package untie

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sort"

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
	diagnostics []source.Diagnostic
	key         uint64
}

// Parse parses source into an expression. Malformed source produces diagnostics
// and a drawable fallback tree rather than an ordinary error.
func Parse(src string, dialect Dialect, mode Mode, opts ParseOptions) *Expr {
	limits := opts.Limits.WithDefaults()
	diagnostics := make([]source.Diagnostic, 0, 2)
	root := fallbackFormula(src)

	if len(src) > limits.MaxSourceBytes {
		diagnostics = append(diagnostics, source.Diagnostic{
			Range:    source.NewRange(0, len(src)),
			Severity: source.SeverityError,
			Code:     source.CodeLimitExceeded,
			Message:  "source exceeds configured byte limit",
		})
	} else {
		switch dialect {
		case TeX:
			macros := make(map[string]tex.Macro, len(opts.Macros))
			for name, macro := range opts.Macros {
				macros[name] = tex.Macro{Body: macro.Body, Args: macro.Args}
			}
			result := tex.Parse(src, mode, tex.Options{Macros: macros, Limits: limits})
			root = &ir.Formula{Root: result.Root}
			diagnostics = append(diagnostics, result.Diagnostics...)
		case Typst:
			result := typst.Parse(src, mode, typst.Options{Preamble: opts.Preamble, Limits: limits})
			root = &ir.Formula{Root: result.Root}
			diagnostics = append(diagnostics, result.Diagnostics...)
		default:
			span := source.NewRange(0, len(src))
			diagnostics = append(diagnostics, source.Diagnostic{
				Range:    span,
				Severity: source.SeverityError,
				Code:     source.CodeInvalidValue,
				Message:  "unknown math dialect",
			})
		}
	}

	expr := &Expr{
		source:      src,
		dialect:     dialect,
		mode:        mode,
		root:        root,
		diagnostics: source.NormalizeDiagnostics(diagnostics),
	}
	expr.key = computeKey(expr, opts)
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

// Root returns the canonical IR. Callers must treat it as read-only.
func (e *Expr) Root() *ir.Formula {
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

// LayoutContext lays out an expression. The layout backend is installed in a
// later work package; the initial implementation reports that clearly.
func LayoutContext(ctx context.Context, e *Expr, fonts *FontSet, style Style) (*Box, error) {
	if err := ctx.Err(); err != nil {
		return &display.Box{}, err
	}
	return &display.Box{}, nil
}

// Layout lays out an expression with a background context.
func Layout(e *Expr, fonts *FontSet, style Style) *Box {
	box, err := LayoutContext(context.Background(), e, fonts, style)
	if err != nil {
		return &display.Box{}
	}
	return box
}

func fallbackFormula(src string) *ir.Formula {
	span := source.NewRange(0, len(src))
	row := &ir.Row{Base: ir.NewBase(ir.KindRow, span)}
	if src != "" {
		row.Items = []ir.Node{ir.NewError(span, src, source.CodeUnsupported)}
	}
	return &ir.Formula{Root: row}
}

func computeKey(e *Expr, opts ParseOptions) uint64 {
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
