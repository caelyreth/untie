package untie

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	internalalt "github.com/caelyreth/untie/internal/alt"
	"github.com/caelyreth/untie/internal/irarena"
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
	Limits           = source.Limits
	Range            = source.Range
	Diagnostic       = source.Diagnostic
	Severity         = source.Severity
	Box              = display.Box
	LineBox          = display.LineBox
	ItemRange        = display.ItemRange
	HitRange         = display.HitRange
	Alt              = display.Alt
	AltDescription   = display.AltDescription
	AltNode          = display.AltNode
	AltNodeKind      = display.AltNodeKind
	AltPart          = display.AltPart
	AltPartRole      = display.AltPartRole
	AltRow           = display.AltRow
	AltColumn        = display.AltColumn
	AltColumnAlign   = display.AltColumnAlign
	AltTag           = display.AltTag
	AltVariant       = display.AltVariant
	AltFractionRole  = display.AltFractionRole
	AltFractionStyle = display.AltFractionStyle
	AltScriptRole    = display.AltScriptRole
	AltTableRole     = display.AltTableRole
	AltCancelRole    = display.AltCancelRole
	AltPosition      = display.AltPosition
	AltSpaceKind     = display.AltSpaceKind
	AltSpaceWidth    = display.AltSpaceWidth
	AltBreakKind     = display.AltBreakKind
	AltSizingKind    = display.AltSizingKind
	AltMathSize      = display.AltMathSize
	AltFlag          = display.AltFlag
	Item             = display.Item
	ItemKind         = display.ItemKind
	Paint            = display.Paint
	Role             = display.Role
	RGBA             = display.RGBA
	ColorPolicy      = display.ColorPolicy
	ColorPolicyMode  = display.ColorPolicyMode
	ColorRolePalette = display.ColorRolePalette
	PixelGrid        = display.PixelGrid
	SnapAxis         = display.SnapAxis
	FontUsage        = display.FontUsage
	FaceUse          = display.FaceUse
	MissingFallback  = display.MissingFallback
	Hit              = display.Hit
	HitKind          = display.HitKind
	Break            = display.Break
	CaretStop        = display.CaretStop
	TagBox           = display.TagBox
	BreakKind        = display.BreakKind
	Align            = display.Align
	FontFace         = font.FontFace
	FontSetConfig    = font.FontSetConfig
	FontSet          = font.FontSet
	FontOption       = font.FontOption
	Formula          = ir.Formula
	EquationTag      = ir.EquationTag
	NodeID           = ir.NodeID

	Point   = geometry.Point
	Rect    = geometry.Rect
	Segment = geometry.Segment
	Path    = geometry.Path

	PathTemplate = display.PathTemplate
)

// Style is a display.Style layout request. Display layout with a positive
// MaxWidth emits Box.Lines in a MaxWidth-wide alignment frame. Align positions
// each line within that frame. Box.Items and Box.Hits are the sole drawable
// storage; LineBox and TagBox ranges select parent-frame records and must not
// be translated again. A formula-level display tag populates Box.Tag and the
// associated LineBox.Tag. Source-row tags populate Box.Tags and use
// TagBox.Line instead. Constrained alignment reserves the applicable tag column
// before positioning the line. Inline layout ignores both MaxWidth and Align
// and rejects manual tags.
type Style = display.Style

const (
	SeverityError   = source.SeverityError
	SeverityWarning = source.SeverityWarning
	SeverityInfo    = source.SeverityInfo

	ItemGlyphPath = display.ItemGlyphPath
	ItemRule      = display.ItemRule
	ItemGlyphRef  = display.ItemGlyphRef

	SegMoveTo = geometry.SegMoveTo
	SegLineTo = geometry.SegLineTo
	SegQuadTo = geometry.SegQuadTo
	SegCubeTo = geometry.SegCubeTo
	SegClose  = geometry.SegClose

	RoleInk         = display.RoleInk
	RoleError       = display.RoleError
	RolePlaceholder = display.RolePlaceholder

	ColorPreserve       = display.ColorPreserve
	ColorMapToRoles     = display.ColorMapToRoles
	ColorAdjustContrast = display.ColorAdjustContrast
	DefaultMinContrast  = display.DefaultMinContrast

	SnapNone = display.SnapNone
	SnapX    = display.SnapX
	SnapY    = display.SnapY

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

	FormulaTagLine = display.FormulaTagLine

	AltDescriptionVersion = display.AltDescriptionVersion

	AltKindRow            = display.AltKindRow
	AltKindGlyph          = display.AltKindGlyph
	AltKindText           = display.AltKindText
	AltKindOperator       = display.AltKindOperator
	AltKindSpace          = display.AltKindSpace
	AltKindFraction       = display.AltKindFraction
	AltKindRadical        = display.AltKindRadical
	AltKindScripts        = display.AltKindScripts
	AltKindFenced         = display.AltKindFenced
	AltKindSizedDelimiter = display.AltKindSizedDelimiter
	AltKindAccent         = display.AltKindAccent
	AltKindLine           = display.AltKindLine
	AltKindCancel         = display.AltKindCancel
	AltKindTable          = display.AltKindTable
	AltKindMultiline      = display.AltKindMultiline
	AltKindStyle          = display.AltKindStyle
	AltKindColor          = display.AltKindColor
	AltKindBox            = display.AltKindBox
	AltKindPhantom        = display.AltKindPhantom
	AltKindError          = display.AltKindError
	AltKindPlaceholder    = display.AltKindPlaceholder
	AltKindBreak          = display.AltKindBreak
	AltKindAlignMarker    = display.AltKindAlignMarker

	AltPartItem        = display.AltPartItem
	AltPartBody        = display.AltPartBody
	AltPartNumerator   = display.AltPartNumerator
	AltPartDenominator = display.AltPartDenominator
	AltPartRadicand    = display.AltPartRadicand
	AltPartIndex       = display.AltPartIndex
	AltPartBase        = display.AltPartBase
	AltPartTop         = display.AltPartTop
	AltPartBottom      = display.AltPartBottom
	AltPartTopLeft     = display.AltPartTopLeft
	AltPartBottomLeft  = display.AltPartBottomLeft
	AltPartTopRight    = display.AltPartTopRight
	AltPartBottomRight = display.AltPartBottomRight
	AltPartOpen        = display.AltPartOpen
	AltPartClose       = display.AltPartClose
	AltPartContent     = display.AltPartContent
	AltPartAccent      = display.AltPartAccent

	AltVariantNormal       = display.AltVariantNormal
	AltVariantItalic       = display.AltVariantItalic
	AltVariantBold         = display.AltVariantBold
	AltVariantBoldItalic   = display.AltVariantBoldItalic
	AltVariantDoubleStruck = display.AltVariantDoubleStruck
	AltVariantScript       = display.AltVariantScript
	AltVariantBoldScript   = display.AltVariantBoldScript
	AltVariantFraktur      = display.AltVariantFraktur
	AltVariantBoldFraktur  = display.AltVariantBoldFraktur
	AltVariantSans         = display.AltVariantSans
	AltVariantBoldSans     = display.AltVariantBoldSans
	AltVariantMonospace    = display.AltVariantMonospace

	AltFractionOrdinary = display.AltFractionOrdinary
	AltFractionBinomial = display.AltFractionBinomial
	AltFractionStacked  = display.AltFractionStacked

	AltFractionVertical   = display.AltFractionVertical
	AltFractionNoBar      = display.AltFractionNoBar
	AltFractionSkewed     = display.AltFractionSkewed
	AltFractionHorizontal = display.AltFractionHorizontal

	AltScriptOrdinary   = display.AltScriptOrdinary
	AltScriptLimits     = display.AltScriptLimits
	AltScriptOverlay    = display.AltScriptOverlay
	AltScriptBraceLabel = display.AltScriptBraceLabel

	AltTableMatrix = display.AltTableMatrix
	AltTableArray  = display.AltTableArray
	AltTableCases  = display.AltTableCases
	AltTableVector = display.AltTableVector

	AltCancelStrike   = display.AltCancelStrike
	AltCancelNegation = display.AltCancelNegation

	AltPositionAbove = display.AltPositionAbove
	AltPositionBelow = display.AltPositionBelow

	AltSpaceExplicit  = display.AltSpaceExplicit
	AltSpaceInterword = display.AltSpaceInterword
	AltSpaceIgnored   = display.AltSpaceIgnored

	AltSpaceWidthZero           = display.AltSpaceWidthZero
	AltSpaceWidthThin           = display.AltSpaceWidthThin
	AltSpaceWidthMedium         = display.AltSpaceWidthMedium
	AltSpaceWidthThick          = display.AltSpaceWidthThick
	AltSpaceWidthEn             = display.AltSpaceWidthEn
	AltSpaceWidthQuad           = display.AltSpaceWidthQuad
	AltSpaceWidthDoubleQuad     = display.AltSpaceWidthDoubleQuad
	AltSpaceWidthNegativeThin   = display.AltSpaceWidthNegativeThin
	AltSpaceWidthNegativeMedium = display.AltSpaceWidthNegativeMedium
	AltSpaceWidthNegativeThick  = display.AltSpaceWidthNegativeThick
	AltSpaceWidthCustom         = display.AltSpaceWidthCustom

	AltBreakRelation = display.AltBreakRelation
	AltBreakBinary   = display.AltBreakBinary
	AltBreakExplicit = display.AltBreakExplicit

	AltSizingContent  = display.AltSizingContent
	AltSizingRelative = display.AltSizingRelative
	AltSizingAbsolute = display.AltSizingAbsolute

	AltColumnCenter = display.AltColumnCenter
	AltColumnLeft   = display.AltColumnLeft
	AltColumnRight  = display.AltColumnRight

	AltSizeInherit      = display.AltSizeInherit
	AltSizeDisplay      = display.AltSizeDisplay
	AltSizeText         = display.AltSizeText
	AltSizeScript       = display.AltSizeScript
	AltSizeScriptScript = display.AltSizeScriptScript

	AltFlagBalanced        = display.AltFlagBalanced
	AltFlagBar             = display.AltFlagBar
	AltFlagWeak            = display.AltFlagWeak
	AltFlagStretchy        = display.AltFlagStretchy
	AltFlagNaturalFit      = display.AltFlagNaturalFit
	AltFlagDotless         = display.AltFlagDotless
	AltFlagExactFrameWidth = display.AltFlagExactFrameWidth
	AltFlagCross           = display.AltFlagCross
	AltFlagInverted        = display.AltFlagInverted
	AltFlagCentered        = display.AltFlagCentered
	AltFlagCramped         = display.AltFlagCramped
	AltFlagUncramped       = display.AltFlagUncramped
	AltFlagFill            = display.AltFlagFill
	AltFlagBorder          = display.AltFlagBorder
	AltFlagPhantomWidth    = display.AltFlagPhantomWidth
	AltFlagPhantomHeight   = display.AltFlagPhantomHeight
	AltFlagPhantomDepth    = display.AltFlagPhantomDepth
)

// Macro is a host-provided TeX macro.
type Macro struct {
	Body string
	Args int
}

// ParseOptions configure parsing. TeX macros are ignored by the Typst
// frontend. Typst preamble/code definitions are intentionally unsupported;
// hosts must expand them before calling Parse.
type ParseOptions struct {
	Macros map[string]Macro
	Limits Limits
}

// Expr is an immutable parsed formula.
type Expr struct {
	source      string
	dialect     Dialect
	mode        Mode
	root        *ir.Formula
	limits      source.Limits
	diagnostics []source.Diagnostic
	arena       *irarena.Arena
	stats       ir.Stats
	key         uint64
}

var (
	// ErrInvalidExpr reports a nil expression.
	ErrInvalidExpr = errors.New("untie: nil expression")
	// ErrAltInvalidRoot reports malformed alternative-text input.
	ErrAltInvalidRoot = internalalt.ErrInvalidRoot
	// ErrAltLimitExceeded reports an alternative-text traversal or output limit.
	ErrAltLimitExceeded = internalalt.ErrLimitExceeded
	// ErrAltUnsupportedNode reports an IR node that alternative text cannot explain.
	ErrAltUnsupportedNode = internalalt.ErrUnsupportedNode
)

// Parse parses source into an expression. Malformed source produces diagnostics
// and a drawable fallback tree rather than an ordinary error. Links are
// text-only: the visible body is rendered with its source spans preserved, the
// destination is discarded, and a warning is reported only after the link
// arguments validate. TeX URL groups preserve raw URL characters and balanced
// braces; Typst link destinations are limited to non-empty strings in the
// bounded evaluator. Labels, references, and document-level link resolution are
// outside this API.
func Parse(src string, dialect Dialect, mode Mode, opts ParseOptions) *Expr {
	expr, _ := ParseContext(context.Background(), src, dialect, mode, opts)
	return expr
}

// ParseContext parses source with context cancellation. Cancellation is checked
// before parser dispatch and again before the expression is returned. Parser
// work already in progress is bounded by opts.Limits rather than interrupted at
// arbitrary points. A canceled call returns a nil expression and ctx.Err.
//
// Parse options are snapshotted before parsing. The caller may reuse or mutate
// its macro map and limits after ParseContext returns.
func ParseContext(
	ctx context.Context,
	src string,
	dialect Dialect,
	mode Mode,
	opts ParseOptions,
) (*Expr, error) {
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts = snapshotParseOptions(opts)
	limits := opts.Limits.WithDefaults()
	expr := parseRecovered(src, dialect, mode, opts, limits, func(limits source.Limits) frontendResult {
		switch dialect {
		case TeX:
			macros := make(map[string]tex.Macro, len(opts.Macros))
			for name, macro := range opts.Macros {
				macros[name] = tex.Macro{Body: macro.Body, Args: macro.Args}
			}
			result := tex.Parse(src, mode, tex.Options{Macros: macros, Limits: limits})
			return frontendResult{root: result.Root, tags: result.Tags, diagnostics: result.Diagnostics, arena: result.Arena}
		case Typst:
			result := typst.Parse(src, mode, typst.Options{Limits: limits})
			return frontendResult{root: result.Root, diagnostics: result.Diagnostics, arena: result.Arena}
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return expr, nil
}

func snapshotParseOptions(opts ParseOptions) ParseOptions {
	opts.Limits = opts.Limits.WithDefaults()
	if len(opts.Macros) == 0 {
		opts.Macros = nil
		return opts
	}
	macros := make(map[string]Macro, len(opts.Macros))
	for name, macro := range opts.Macros {
		macros[name] = macro
	}
	opts.Macros = macros
	return opts
}

type frontendResult struct {
	root        ir.Node
	tags        []ir.EquationTag
	diagnostics []source.Diagnostic
	arena       *irarena.Arena
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
	var root *ir.Formula
	var arena *irarena.Arena
	var stats ir.Stats
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
			arena = nil
			stats = ir.Stats{}
			diagnostics = []source.Diagnostic{diagnostic}
			expr = newExpr(src, dialect, mode, limits, root, arena, stats, diagnostics, opts)
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
		candidate := &ir.Formula{Root: result.root, Tags: result.tags}
		if result.root == nil {
			diagnostics = append(diagnostics, result.diagnostics...)
			diagnostics = append(diagnostics, source.Diagnostic{
				Range:    source.NewRange(0, len(src)),
				Severity: source.SeverityError,
				Code:     source.CodeInternal,
				Message:  "frontend returned a nil IR root",
			})
		} else if validated, err := ir.ValidateSourceWithStats(candidate, src, limits); err != nil {
			diagnostics = append(diagnostics, result.diagnostics...)
			code := source.CodeInternal
			message := fmt.Sprintf("frontend returned invalid IR: %v", err)
			if len(src) > limits.MaxTextBytes {
				code = source.CodeLimitExceeded
				message = fmt.Sprintf("frontend output exceeds text byte limit: %v", err)
			}
			diagnostics = append(diagnostics, source.Diagnostic{
				Range:    source.NewRange(0, len(src)),
				Severity: source.SeverityError,
				Code:     code,
				Message:  message,
			})
		} else {
			root = candidate
			arena = result.arena
			stats = validated
			diagnostics = append(diagnostics, result.diagnostics...)
		}
	}

	if root == nil {
		root = fallbackFormula(src, limits)
		stats, _ = ir.ValidateWithStats(root, limits)
	}
	return newExpr(src, dialect, mode, limits, root, arena, stats, diagnostics, opts)
}

func newExpr(
	src string,
	dialect Dialect,
	mode Mode,
	limits source.Limits,
	root *ir.Formula,
	arena *irarena.Arena,
	stats ir.Stats,
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
		arena:       arena,
		stats:       stats,
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

// HasErrors reports whether parsing produced at least one error-severity
// diagnostic. The expression remains drawable whether this returns true or
// false.
func (e *Expr) HasErrors() bool {
	if e == nil {
		return false
	}
	for _, diagnostic := range e.diagnostics {
		if diagnostic.Severity == source.SeverityError {
			return true
		}
	}
	return false
}

// Alt returns deterministic plain text, speech, and a structured localization
// tree derived from the neutral IR. If generation cannot complete, it returns
// a bounded, validated fallback. Use AltContext to observe the error.
func (e *Expr) Alt() Alt {
	if e == nil {
		return Alt{}
	}
	out, _ := e.AltContext(context.Background())
	return out
}

// AltContext returns alternative text and its structured accessibility
// description with cooperative cancellation.
//
// Nil contexts and cancellation return a zero Alt. Non-context failures return
// a bounded, validated visible fallback together with an error. Unsupported
// nested nodes may return partially generated output containing visible
// markers. Use ErrAltInvalidRoot, ErrAltLimitExceeded, and
// ErrAltUnsupportedNode to classify those errors.
func (e *Expr) AltContext(ctx context.Context) (Alt, error) {
	if ctx == nil {
		return Alt{}, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return Alt{}, err
	}
	if e == nil || e.root == nil || e.root.Root == nil {
		return Alt{}, ErrInvalidExpr
	}
	return internalalt.GenerateFormulaContext(ctx, e.root, e.mode, e.limits)
}

// Key returns a stable convenience cache key for the parsed expression.
func (e *Expr) Key() uint64 {
	if e == nil {
		return 0
	}
	return e.key
}

// LoadFontSet prepares one or more host-supplied faces for repeated layout.
// Empty face IDs are derived deterministically.
func LoadFontSet(config FontSetConfig) (*FontSet, error) {
	prepared, err := font.LoadFontSet(config)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFonts, err)
	}
	return prepared, nil
}

// LoadMathFont prepares one math face. Use WithFontID and WithFontIndex when
// the host needs explicit identity or a TTC face index.
func LoadMathFont(data []byte, opts ...FontOption) (*FontSet, error) {
	prepared, err := font.LoadMathFont(data, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFonts, err)
	}
	return prepared, nil
}

// WithFontID sets an explicit face ID for LoadMathFont.
func WithFontID(id string) FontOption {
	return font.WithFontID(id)
}

// WithFontIndex selects a TTC face for LoadMathFont.
func WithFontIndex(index int) FontOption {
	return font.WithFontIndex(index)
}

func fallbackFormula(src string, limits source.Limits) *ir.Formula {
	return &ir.Formula{Root: fallbackRoot(src, limits)}
}

func fallbackRoot(src string, limits source.Limits) ir.Node {
	span := source.NewRange(0, len(src))
	text := ""
	if src != "" {
		text = truncateUTF8(src, limits.MaxTextBytes)
		if text == "" {
			text = "?"
		}
	}
	if text != "" && limits.MaxNodes <= 1 {
		return ir.NewError(span, text, source.CodeUnsupported)
	}
	row := &ir.Row{Base: ir.NewBase(ir.KindRow, span)}
	if text != "" {
		row.Items = []ir.Node{ir.NewError(span, text, source.CodeUnsupported)}
	}
	return row
}

func truncateUTF8(text string, limit int) string {
	if limit <= 0 {
		limit = 1
	}
	if len(text) <= limit {
		return text
	}
	text = text[:limit]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	if text == "" {
		return "?"
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
	for _, limit := range []int{
		limits.MaxSourceBytes,
		limits.MaxTokens,
		limits.MaxNesting,
		limits.MaxNodes,
		limits.MaxMacroExpansions,
		limits.MaxExpandedTokens,
		limits.MaxCallDepth,
		limits.MaxTableCells,
		limits.MaxTextBytes,
	} {
		writeInt(int64(limit))
	}
	writeInt(limits.MaxDimension)

	if e.dialect == TeX {
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
	}
	sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(sum[:8])
}
