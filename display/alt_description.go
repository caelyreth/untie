package display

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

// AltDescriptionVersion is the current structured accessibility schema.
const AltDescriptionVersion = 2

// ErrInvalidAltDescription reports a malformed structured accessibility
// description.
var ErrInvalidAltDescription = errors.New("display: invalid alternative description")

// AltNodeKind identifies one semantic node in a structured accessibility
// description. Values are stable protocol tokens, not user-facing prose.
type AltNodeKind string

const (
	AltKindRow            AltNodeKind = "row"
	AltKindGlyph          AltNodeKind = "glyph"
	AltKindText           AltNodeKind = "text"
	AltKindOperator       AltNodeKind = "operator"
	AltKindSpace          AltNodeKind = "space"
	AltKindFraction       AltNodeKind = "fraction"
	AltKindRadical        AltNodeKind = "radical"
	AltKindScripts        AltNodeKind = "scripts"
	AltKindFenced         AltNodeKind = "fenced"
	AltKindSizedDelimiter AltNodeKind = "sized-delimiter"
	AltKindAccent         AltNodeKind = "accent"
	AltKindLine           AltNodeKind = "line"
	AltKindCancel         AltNodeKind = "cancel"
	AltKindTable          AltNodeKind = "table"
	AltKindMultiline      AltNodeKind = "multiline"
	AltKindStyle          AltNodeKind = "style"
	AltKindOverlay        AltNodeKind = "overlay"
	AltKindRaise          AltNodeKind = "raise"
	AltKindColor          AltNodeKind = "color"
	AltKindBox            AltNodeKind = "box"
	AltKindPhantom        AltNodeKind = "phantom"
	AltKindError          AltNodeKind = "error"
	AltKindPlaceholder    AltNodeKind = "placeholder"
	AltKindBreak          AltNodeKind = "break"
	AltKindAlignMarker    AltNodeKind = "align-marker"
)

// AltPartRole names the semantic position of a child node.
type AltPartRole string

const (
	AltPartItem        AltPartRole = "item"
	AltPartBody        AltPartRole = "body"
	AltPartNumerator   AltPartRole = "numerator"
	AltPartDenominator AltPartRole = "denominator"
	AltPartRadicand    AltPartRole = "radicand"
	AltPartIndex       AltPartRole = "index"
	AltPartBase        AltPartRole = "base"
	AltPartTop         AltPartRole = "top"
	AltPartBottom      AltPartRole = "bottom"
	AltPartTopLeft     AltPartRole = "top-left"
	AltPartBottomLeft  AltPartRole = "bottom-left"
	AltPartTopRight    AltPartRole = "top-right"
	AltPartBottomRight AltPartRole = "bottom-right"
	AltPartOpen        AltPartRole = "open"
	AltPartClose       AltPartRole = "close"
	AltPartContent     AltPartRole = "content"
	AltPartAccent      AltPartRole = "accent"
)

// AltVariant identifies a semantic math alphabet.
type AltVariant string

const (
	AltVariantNormal       AltVariant = "normal"
	AltVariantItalic       AltVariant = "italic"
	AltVariantBold         AltVariant = "bold"
	AltVariantBoldItalic   AltVariant = "bold-italic"
	AltVariantDoubleStruck AltVariant = "double-struck"
	AltVariantScript       AltVariant = "script"
	AltVariantBoldScript   AltVariant = "bold-script"
	AltVariantFraktur      AltVariant = "fraktur"
	AltVariantBoldFraktur  AltVariant = "bold-fraktur"
	AltVariantSans         AltVariant = "sans-serif"
	AltVariantBoldSans     AltVariant = "bold-sans-serif"
	AltVariantMonospace    AltVariant = "monospace"
)

// AltFractionRole identifies the semantic operation represented by a
// fraction-shaped node.
type AltFractionRole string

const (
	AltFractionOrdinary AltFractionRole = "ordinary"
	AltFractionBinomial AltFractionRole = "binomial"
	AltFractionStacked  AltFractionRole = "stacked"
)

// AltFractionStyle identifies the geometric fraction construction.
type AltFractionStyle string

const (
	AltFractionVertical   AltFractionStyle = "vertical"
	AltFractionNoBar      AltFractionStyle = "no-bar"
	AltFractionSkewed     AltFractionStyle = "skewed"
	AltFractionHorizontal AltFractionStyle = "horizontal"
)

// AltScriptRole identifies the semantic purpose of scripts and limits.
type AltScriptRole string

const (
	AltScriptOrdinary   AltScriptRole = "ordinary"
	AltScriptLimits     AltScriptRole = "limits"
	AltScriptOverlay    AltScriptRole = "overlay"
	AltScriptBraceLabel AltScriptRole = "brace-label"
)

// AltTableRole identifies the semantic kind of a table.
type AltTableRole string

const (
	AltTableMatrix AltTableRole = "matrix"
	AltTableArray  AltTableRole = "array"
	AltTableCases  AltTableRole = "cases"
	AltTableVector AltTableRole = "vector"
)

// AltCancelRole distinguishes cancellation, negation, and Feynman slash.
type AltCancelRole string

const (
	AltCancelStrike   AltCancelRole = "strike"
	AltCancelNegation AltCancelRole = "negation"
	AltCancelSlashed  AltCancelRole = "slashed"
)

// AltPosition identifies an above/below decoration.
type AltPosition string

const (
	AltPositionAbove AltPosition = "above"
	AltPositionBelow AltPosition = "below"
)

// AltSpaceKind identifies the semantic kind of an explicit or interword space.
type AltSpaceKind string

const (
	AltSpaceExplicit  AltSpaceKind = "explicit"
	AltSpaceInterword AltSpaceKind = "interword"
	AltSpaceIgnored   AltSpaceKind = "ignored"
)

// AltSpaceWidth identifies the authored width of an explicit space when it
// matches a stable semantic width.
type AltSpaceWidth string

const (
	AltSpaceWidthZero           AltSpaceWidth = "zero"
	AltSpaceWidthThin           AltSpaceWidth = "thin"
	AltSpaceWidthMedium         AltSpaceWidth = "medium"
	AltSpaceWidthThick          AltSpaceWidth = "thick"
	AltSpaceWidthEn             AltSpaceWidth = "en"
	AltSpaceWidthQuad           AltSpaceWidth = "quad"
	AltSpaceWidthDoubleQuad     AltSpaceWidth = "double-quad"
	AltSpaceWidthNegativeThin   AltSpaceWidth = "negative-thin"
	AltSpaceWidthNegativeMedium AltSpaceWidth = "negative-medium"
	AltSpaceWidthNegativeThick  AltSpaceWidth = "negative-thick"
	AltSpaceWidthCustom         AltSpaceWidth = "custom"
)

// AltBreakKind identifies why a line break is legal.
type AltBreakKind string

const (
	AltBreakRelation AltBreakKind = "relation"
	AltBreakBinary   AltBreakKind = "binary"
	AltBreakExplicit AltBreakKind = "explicit"
)

// AltSizingKind identifies a delimiter target policy.
type AltSizingKind string

const (
	AltSizingContent  AltSizingKind = "content"
	AltSizingRelative AltSizingKind = "relative"
	AltSizingAbsolute AltSizingKind = "absolute"
)

// AltColumnAlign identifies a table column alignment.
type AltColumnAlign string

const (
	AltColumnCenter AltColumnAlign = "center"
	AltColumnLeft   AltColumnAlign = "left"
	AltColumnRight  AltColumnAlign = "right"
)

// AltMathSize identifies an explicit math-size wrapper.
type AltMathSize string

const (
	AltSizeInherit      AltMathSize = "inherit"
	AltSizeDisplay      AltMathSize = "display"
	AltSizeText         AltMathSize = "text"
	AltSizeScript       AltMathSize = "script"
	AltSizeScriptScript AltMathSize = "script-script"
)

// AltOverlayAlignment identifies the horizontal placement of zero-width
// overlay content.
type AltOverlayAlignment string

const (
	AltOverlayStart  AltOverlayAlignment = "start"
	AltOverlayCenter AltOverlayAlignment = "center"
	AltOverlayEnd    AltOverlayAlignment = "end"
)

// AltRaiseDirection identifies the semantic vertical direction of a raise.
type AltRaiseDirection string

const (
	AltRaiseUp   AltRaiseDirection = "up"
	AltRaiseDown AltRaiseDirection = "down"
	AltRaiseZero AltRaiseDirection = "zero"
)

// AltFontSize is an explicit uniform font-size percentage relative to the
// root host font size. A value of 100 means 100 percent.
type AltFontSize struct {
	Percent float64 `json:"percent"`
}

// AltFlag carries a stable semantic boolean. Absence means false; a defined
// positive or negative token records the authored state where needed.
type AltFlag string

const (
	AltFlagBalanced        AltFlag = "balanced"
	AltFlagBar             AltFlag = "bar"
	AltFlagWeak            AltFlag = "weak"
	AltFlagStretchy        AltFlag = "stretchy"
	AltFlagNaturalFit      AltFlag = "natural-fit"
	AltFlagDotless         AltFlag = "dotless"
	AltFlagExactFrameWidth AltFlag = "exact-frame-width"
	AltFlagCross           AltFlag = "cross"
	AltFlagInverted        AltFlag = "inverted"
	AltFlagCentered        AltFlag = "centered"
	AltFlagCramped         AltFlag = "cramped"
	AltFlagUncramped       AltFlag = "uncramped"
	AltFlagFill            AltFlag = "fill"
	AltFlagBorder          AltFlag = "border"
	AltFlagPhantomWidth    AltFlag = "phantom-width"
	AltFlagPhantomHeight   AltFlag = "phantom-height"
	AltFlagPhantomDepth    AltFlag = "phantom-depth"
)

// AltDescription is a stable, source-independent semantic description derived
// from canonical IR. It contains no source ranges, node identities, paint,
// fonts, or geometry. Hosts map its tokens and literal text payloads to their
// own locale and accessibility vocabulary.
type AltDescription struct {
	Version int      `json:"version"`
	Root    AltNode  `json:"root"`
	Tags    []AltTag `json:"tags,omitempty"`
}

// AltNode is one semantic node. Kind determines which optional fields and
// child collections are meaningful.
type AltNode struct {
	Kind AltNodeKind `json:"kind"`

	// Text is literal source-independent content for glyph, text, operator,
	// delimiter, error, and placeholder nodes. It is not localized prose.
	Text string `json:"text,omitempty"`
	// Code is a stable error or diagnostic token for error nodes.
	Code string `json:"code,omitempty"`

	Variant          AltVariant          `json:"variant,omitempty"`
	FractionRole     AltFractionRole     `json:"fractionRole,omitempty"`
	FractionStyle    AltFractionStyle    `json:"fractionStyle,omitempty"`
	ScriptRole       AltScriptRole       `json:"scriptRole,omitempty"`
	TableRole        AltTableRole        `json:"tableRole,omitempty"`
	CancelRole       AltCancelRole       `json:"cancelRole,omitempty"`
	Position         AltPosition         `json:"position,omitempty"`
	Space            AltSpaceKind        `json:"space,omitempty"`
	SpaceWidth       AltSpaceWidth       `json:"spaceWidth,omitempty"`
	Break            AltBreakKind        `json:"break,omitempty"`
	Sizing           AltSizingKind       `json:"sizing,omitempty"`
	Size             AltMathSize         `json:"size,omitempty"`
	OverlayAlignment AltOverlayAlignment `json:"overlayAlignment,omitempty"`
	RaiseDirection   AltRaiseDirection   `json:"raise,omitempty"`
	FontSize         *AltFontSize        `json:"fontSize,omitempty"`

	Flags   []AltFlag   `json:"flags,omitempty"`
	Parts   []AltPart   `json:"parts,omitempty"`
	Rows    []AltRow    `json:"rows,omitempty"`
	Columns []AltColumn `json:"columns,omitempty"`
}

// AltPart attaches a child node to a named semantic position.
type AltPart struct {
	Role AltPartRole `json:"role"`
	Node AltNode     `json:"node"`
}

// AltRow is one table or multiline row. Items are cells for tables and
// row-level components for multiline expressions.
type AltRow struct {
	Items []AltNode `json:"items,omitempty"`
}

// AltColumn records one table column's alignment.
type AltColumn struct {
	Align AltColumnAlign `json:"align"`
}

// AltTag describes one formula- or row-scoped tag event. A suppressed event
// has no body. Line is FormulaTagLine for the complete formula or the
// zero-based source row for a multiline row tag.
type AltTag struct {
	Body       *AltNode `json:"body,omitempty"`
	Manual     bool     `json:"manual,omitempty"`
	Starred    bool     `json:"starred,omitempty"`
	Suppressed bool     `json:"suppressed,omitempty"`
	Line       int      `json:"line"`
}

// Clone returns an independent copy of d.
func (d *AltDescription) Clone() *AltDescription {
	if d == nil {
		return nil
	}
	out := *d
	out.Root = d.Root.Clone()
	if len(d.Tags) > 0 {
		out.Tags = make([]AltTag, len(d.Tags))
		for i, tag := range d.Tags {
			out.Tags[i] = tag
			if tag.Body != nil {
				body := tag.Body.Clone()
				out.Tags[i].Body = &body
			}
		}
	}
	return &out
}

// Clone returns a deep copy of n.
func (n AltNode) Clone() AltNode {
	out := n
	out.Flags = append([]AltFlag(nil), n.Flags...)
	if n.FontSize != nil {
		fontSize := *n.FontSize
		out.FontSize = &fontSize
	}
	if len(n.Parts) > 0 {
		out.Parts = make([]AltPart, len(n.Parts))
		for i, part := range n.Parts {
			out.Parts[i] = AltPart{Role: part.Role, Node: part.Node.Clone()}
		}
	}
	if len(n.Rows) > 0 {
		out.Rows = make([]AltRow, len(n.Rows))
		for i, row := range n.Rows {
			out.Rows[i].Items = cloneAltNodes(row.Items)
		}
	}
	out.Columns = append([]AltColumn(nil), n.Columns...)
	return out
}

func cloneAltNodes(nodes []AltNode) []AltNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]AltNode, len(nodes))
	for i, node := range nodes {
		out[i] = node.Clone()
	}
	return out
}

// Equal reports whether two structured descriptions are structurally equal.
func (d *AltDescription) Equal(other *AltDescription) bool {
	return reflect.DeepEqual(d, other)
}

// Validate checks the default-limit structural contract.
func (d *AltDescription) Validate() error {
	return d.ValidateWithLimits(source.Limits{})
}

// ValidateWithLimits checks the version, semantic shape, text encoding, node
// and nesting budgets, and aggregate literal-text budget. A zero-value limits
// uses source.DefaultLimits.
func (d *AltDescription) ValidateWithLimits(limits source.Limits) error {
	if d == nil {
		return fmt.Errorf("%w: nil description", ErrInvalidAltDescription)
	}
	limits = limits.WithDefaults()
	if d.Version != AltDescriptionVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidAltDescription, d.Version)
	}
	state := altDescriptionValidationState{limits: limits}
	if err := state.node(d.Root, 0, "root"); err != nil {
		return err
	}
	hasFormulaTag := false
	hasRowTag := false
	previousRowLine := -1
	rowLines := make(map[int]bool)
	for i, tag := range d.Tags {
		path := fmt.Sprintf("tags[%d]", i)
		if tag.Line < -1 {
			return fmt.Errorf("%w: %s.line %d", ErrInvalidAltDescription, path, tag.Line)
		}
		if tag.Line == FormulaTagLine {
			if hasRowTag {
				return fmt.Errorf("%w: %s mixes formula and row tags", ErrInvalidAltDescription, path)
			}
			hasFormulaTag = true
		} else {
			if hasFormulaTag {
				return fmt.Errorf("%w: %s mixes formula and row tags", ErrInvalidAltDescription, path)
			}
			if tag.Line <= previousRowLine {
				return fmt.Errorf("%w: %s row tags are not strictly ordered", ErrInvalidAltDescription, path)
			}
			if rowLines[tag.Line] {
				return fmt.Errorf("%w: %s duplicate row tag", ErrInvalidAltDescription, path)
			}
			hasRowTag = true
			previousRowLine = tag.Line
			rowLines[tag.Line] = true
		}
		if tag.Starred && !tag.Manual {
			return fmt.Errorf("%w: %s.starred without manual", ErrInvalidAltDescription, path)
		}
		if tag.Suppressed {
			if tag.Body != nil {
				return fmt.Errorf("%w: %s.suppressed has a body", ErrInvalidAltDescription, path)
			}
			if tag.Manual || tag.Starred {
				return fmt.Errorf("%w: %s.suppressed cannot be manual or starred", ErrInvalidAltDescription, path)
			}
		} else {
			if !tag.Manual {
				return fmt.Errorf("%w: %s non-suppressed tag is not manual", ErrInvalidAltDescription, path)
			}
			if tag.Body == nil {
				return fmt.Errorf("%w: %s body is nil", ErrInvalidAltDescription, path)
			}
		}
		state.count++
		if state.count > state.limits.MaxNodes {
			return fmt.Errorf("%w: node limit %d", ErrInvalidAltDescription, state.limits.MaxNodes)
		}
		if tag.Body != nil {
			if err := state.node(*tag.Body, 0, path+".body"); err != nil {
				return err
			}
		}
	}
	if state.textBytes > state.limits.MaxTextBytes {
		return fmt.Errorf("%w: text payload exceeds %d bytes",
			ErrInvalidAltDescription, state.limits.MaxTextBytes)
	}
	return nil
}

// MarshalCanonical returns deterministic JSON for a validated structured
// accessibility description. It is an inspection and golden-test serializer,
// not an unversioned wire protocol.
func (d *AltDescription) MarshalCanonical() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(d)
}

type altDescriptionValidationState struct {
	limits    source.Limits
	count     int
	textBytes int
}

func (s *altDescriptionValidationState) node(n AltNode, depth int, path string) error {
	s.count++
	if s.count > s.limits.MaxNodes {
		return fmt.Errorf("%w: %s node limit %d",
			ErrInvalidAltDescription, path, s.limits.MaxNodes)
	}
	if depth > s.limits.MaxNesting {
		return fmt.Errorf("%w: %s nesting limit %d",
			ErrInvalidAltDescription, path, s.limits.MaxNesting)
	}
	if !n.Kind.valid() {
		return fmt.Errorf("%w: %s kind %q", ErrInvalidAltDescription, path, n.Kind)
	}
	if err := validateAltText(n.Text, path+".text"); err != nil {
		return err
	}
	s.textBytes += len(n.Text)
	if s.textBytes > s.limits.MaxTextBytes {
		return fmt.Errorf("%w: text payload exceeds %d bytes",
			ErrInvalidAltDescription, s.limits.MaxTextBytes)
	}
	if n.Code != "" {
		if err := validateAltText(n.Code, path+".code"); err != nil {
			return err
		}
		if strings.TrimSpace(n.Code) == "" {
			return fmt.Errorf("%w: %s.code is blank", ErrInvalidAltDescription, path)
		}
	}
	if n.Variant != "" && !n.Variant.valid() {
		return fmt.Errorf("%w: %s variant %q", ErrInvalidAltDescription, path, n.Variant)
	}
	for _, field := range []struct {
		name  string
		value bool
	}{
		{"fractionRole", n.FractionRole != "" && !n.FractionRole.valid()},
		{"fractionStyle", n.FractionStyle != "" && !n.FractionStyle.valid()},
		{"scriptRole", n.ScriptRole != "" && !n.ScriptRole.valid()},
		{"tableRole", n.TableRole != "" && !n.TableRole.valid()},
		{"cancelRole", n.CancelRole != "" && !n.CancelRole.valid()},
		{"position", n.Position != "" && !n.Position.valid()},
		{"space", n.Space != "" && !n.Space.valid()},
		{"spaceWidth", n.SpaceWidth != "" && !n.SpaceWidth.valid()},
		{"break", n.Break != "" && !n.Break.valid()},
		{"sizing", n.Sizing != "" && !n.Sizing.valid()},
		{"size", n.Size != "" && !n.Size.valid()},
		{"overlayAlignment", n.OverlayAlignment != "" && !n.OverlayAlignment.valid()},
		{"raise", n.RaiseDirection != "" && !n.RaiseDirection.valid()},
	} {
		name := field.name
		value := field.value
		if value {
			return fmt.Errorf("%w: %s has invalid %s", ErrInvalidAltDescription, path, name)
		}
	}
	seenFlags := make(map[AltFlag]bool, len(n.Flags))
	for _, flag := range n.Flags {
		if !flag.valid() {
			return fmt.Errorf("%w: %s flag %q", ErrInvalidAltDescription, path, flag)
		}
		if seenFlags[flag] {
			return fmt.Errorf("%w: %s duplicate flag %q", ErrInvalidAltDescription, path, flag)
		}
		seenFlags[flag] = true
	}
	if n.FontSize != nil {
		if math.IsNaN(n.FontSize.Percent) || math.IsInf(n.FontSize.Percent, 0) ||
			n.FontSize.Percent <= 0 {
			return fmt.Errorf("%w: %s font size is invalid", ErrInvalidAltDescription, path)
		}
	}
	if err := n.validateShape(path); err != nil {
		return err
	}
	for i, part := range n.Parts {
		if !part.Role.valid() {
			return fmt.Errorf("%w: %s.parts[%d] role %q",
				ErrInvalidAltDescription, path, i, part.Role)
		}
		if err := s.node(part.Node, depth+1, fmt.Sprintf("%s.parts[%d].node", path, i)); err != nil {
			return err
		}
	}
	for i, row := range n.Rows {
		for j, item := range row.Items {
			if err := s.node(item, depth+1, fmt.Sprintf("%s.rows[%d].items[%d]", path, i, j)); err != nil {
				return err
			}
		}
	}
	for i, column := range n.Columns {
		if !column.Align.valid() {
			return fmt.Errorf("%w: %s.columns[%d] align %q",
				ErrInvalidAltDescription, path, i, column.Align)
		}
	}
	return nil
}

func (n AltNode) validateShape(path string) error {
	allowedPart := map[AltPartRole]bool{}
	requireParts := map[AltPartRole]int{}
	allowRows := false
	allowColumns := false
	allowText := false
	allowCode := n.Kind == AltKindError
	allowVariant := n.Kind == AltKindGlyph || n.Kind == AltKindText ||
		n.Kind == AltKindOperator || n.Kind == AltKindSizedDelimiter
	allowFlags := map[AltFlag]bool{}

	switch n.Kind {
	case AltKindRow:
		allowedPart[AltPartItem] = true
	case AltKindGlyph, AltKindText, AltKindOperator:
		allowText = true
	case AltKindSpace:
		if n.Space == "" {
			return fmt.Errorf("%w: %s space is empty", ErrInvalidAltDescription, path)
		}
		if n.Space == AltSpaceExplicit && n.SpaceWidth == "" {
			return fmt.Errorf("%w: %s explicit space width is empty", ErrInvalidAltDescription, path)
		}
		if n.Space != AltSpaceExplicit && n.SpaceWidth != "" {
			return fmt.Errorf("%w: %s non-explicit space has a width", ErrInvalidAltDescription, path)
		}
		allowFlags[AltFlagWeak] = true
	case AltKindFraction:
		if n.FractionRole == "" || n.FractionStyle == "" {
			return fmt.Errorf("%w: %s fraction role/style is empty", ErrInvalidAltDescription, path)
		}
		allowedPart[AltPartNumerator] = true
		allowedPart[AltPartDenominator] = true
		requireParts[AltPartNumerator] = 1
		requireParts[AltPartDenominator] = 1
		allowFlags[AltFlagBar] = true
	case AltKindRadical:
		allowedPart[AltPartRadicand] = true
		allowedPart[AltPartIndex] = true
		requireParts[AltPartRadicand] = 1
	case AltKindScripts:
		if n.ScriptRole == "" {
			return fmt.Errorf("%w: %s script role is empty", ErrInvalidAltDescription, path)
		}
		for _, role := range []AltPartRole{
			AltPartBase, AltPartTop, AltPartBottom, AltPartTopLeft,
			AltPartBottomLeft, AltPartTopRight, AltPartBottomRight,
		} {
			allowedPart[role] = true
		}
		requireParts[AltPartBase] = 1
	case AltKindFenced:
		allowedPart[AltPartOpen] = true
		allowedPart[AltPartClose] = true
		allowedPart[AltPartContent] = true
		allowFlags[AltFlagBalanced] = true
	case AltKindSizedDelimiter:
		allowText = true
		if n.Sizing == "" {
			return fmt.Errorf("%w: %s sizing is empty", ErrInvalidAltDescription, path)
		}
	case AltKindAccent:
		allowedPart[AltPartBody] = true
		allowedPart[AltPartAccent] = true
		requireParts[AltPartBody] = 1
		requireParts[AltPartAccent] = 1
		if n.Position == "" {
			return fmt.Errorf("%w: %s position is empty", ErrInvalidAltDescription, path)
		}
		allowFlags[AltFlagStretchy] = true
		allowFlags[AltFlagNaturalFit] = true
		allowFlags[AltFlagDotless] = true
		allowFlags[AltFlagExactFrameWidth] = true
	case AltKindLine:
		allowedPart[AltPartBody] = true
		requireParts[AltPartBody] = 1
		if n.Position == "" {
			return fmt.Errorf("%w: %s position is empty", ErrInvalidAltDescription, path)
		}
	case AltKindCancel:
		allowedPart[AltPartBody] = true
		requireParts[AltPartBody] = 1
		if n.CancelRole == "" {
			return fmt.Errorf("%w: %s cancel role is empty", ErrInvalidAltDescription, path)
		}
		allowFlags[AltFlagCross] = true
		allowFlags[AltFlagInverted] = true
	case AltKindTable:
		allowRows = true
		allowColumns = true
		if n.TableRole == "" {
			return fmt.Errorf("%w: %s table role is empty", ErrInvalidAltDescription, path)
		}
	case AltKindMultiline:
		allowRows = true
		allowFlags[AltFlagCentered] = true
	case AltKindStyle:
		allowedPart[AltPartBody] = true
		requireParts[AltPartBody] = 1
		allowFlags[AltFlagCramped] = true
		allowFlags[AltFlagUncramped] = true
	case AltKindOverlay:
		allowedPart[AltPartBody] = true
		requireParts[AltPartBody] = 1
		if n.OverlayAlignment == "" {
			return fmt.Errorf("%w: %s overlay alignment is empty", ErrInvalidAltDescription, path)
		}
	case AltKindRaise:
		allowedPart[AltPartBody] = true
		requireParts[AltPartBody] = 1
		if n.RaiseDirection == "" {
			return fmt.Errorf("%w: %s raise direction is empty", ErrInvalidAltDescription, path)
		}
	case AltKindColor, AltKindBox, AltKindPhantom:
		allowedPart[AltPartBody] = true
		requireParts[AltPartBody] = 1
		if n.Kind == AltKindBox {
			allowFlags[AltFlagFill] = true
			allowFlags[AltFlagBorder] = true
		}
		if n.Kind == AltKindPhantom {
			allowFlags[AltFlagPhantomWidth] = true
			allowFlags[AltFlagPhantomHeight] = true
			allowFlags[AltFlagPhantomDepth] = true
		}
	case AltKindError:
		allowText = true
		if n.Code == "" {
			return fmt.Errorf("%w: %s error code is empty", ErrInvalidAltDescription, path)
		}
	case AltKindPlaceholder:
		allowText = true
	case AltKindBreak:
		if n.Break == "" {
			return fmt.Errorf("%w: %s break kind is empty", ErrInvalidAltDescription, path)
		}
	case AltKindAlignMarker:
	}

	if n.Text != "" && !allowText {
		return fmt.Errorf("%w: %s kind %q has text", ErrInvalidAltDescription, path, n.Kind)
	}
	if n.Code != "" && !allowCode {
		return fmt.Errorf("%w: %s kind %q has code", ErrInvalidAltDescription, path, n.Kind)
	}
	if n.Variant != "" && !allowVariant {
		return fmt.Errorf("%w: %s kind %q has variant", ErrInvalidAltDescription, path, n.Kind)
	}
	if n.FractionRole != "" && n.Kind != AltKindFraction ||
		n.FractionStyle != "" && n.Kind != AltKindFraction {
		return fmt.Errorf("%w: %s has fraction metadata", ErrInvalidAltDescription, path)
	}
	if n.ScriptRole != "" && n.Kind != AltKindScripts {
		return fmt.Errorf("%w: %s has script metadata", ErrInvalidAltDescription, path)
	}
	if n.TableRole != "" && n.Kind != AltKindTable {
		return fmt.Errorf("%w: %s has table metadata", ErrInvalidAltDescription, path)
	}
	if n.CancelRole != "" && n.Kind != AltKindCancel {
		return fmt.Errorf("%w: %s has cancel metadata", ErrInvalidAltDescription, path)
	}
	if n.Position != "" && n.Kind != AltKindAccent && n.Kind != AltKindLine {
		return fmt.Errorf("%w: %s has position metadata", ErrInvalidAltDescription, path)
	}
	if n.Space != "" && n.Kind != AltKindSpace {
		return fmt.Errorf("%w: %s has space metadata", ErrInvalidAltDescription, path)
	}
	if n.SpaceWidth != "" && n.Kind != AltKindSpace {
		return fmt.Errorf("%w: %s has space-width metadata", ErrInvalidAltDescription, path)
	}
	if n.Break != "" && n.Kind != AltKindBreak {
		return fmt.Errorf("%w: %s has break metadata", ErrInvalidAltDescription, path)
	}
	if n.Sizing != "" && n.Kind != AltKindFenced && n.Kind != AltKindSizedDelimiter {
		return fmt.Errorf("%w: %s has sizing metadata", ErrInvalidAltDescription, path)
	}
	if n.Size != "" && n.Kind != AltKindStyle {
		return fmt.Errorf("%w: %s has size metadata", ErrInvalidAltDescription, path)
	}
	if n.OverlayAlignment != "" && n.Kind != AltKindOverlay {
		return fmt.Errorf("%w: %s has overlay-alignment metadata", ErrInvalidAltDescription, path)
	}
	if n.RaiseDirection != "" && n.Kind != AltKindRaise {
		return fmt.Errorf("%w: %s has raise metadata", ErrInvalidAltDescription, path)
	}
	if n.FontSize != nil && n.Kind != AltKindStyle {
		return fmt.Errorf("%w: %s has font-size metadata", ErrInvalidAltDescription, path)
	}
	if len(n.Rows) > 0 && !allowRows {
		return fmt.Errorf("%w: %s kind %q has rows", ErrInvalidAltDescription, path, n.Kind)
	}
	if len(n.Columns) > 0 && !allowColumns {
		return fmt.Errorf("%w: %s kind %q has columns", ErrInvalidAltDescription, path, n.Kind)
	}
	for _, flag := range n.Flags {
		if !allowFlags[flag] {
			return fmt.Errorf("%w: %s kind %q has flag %q",
				ErrInvalidAltDescription, path, n.Kind, flag)
		}
	}
	counts := make(map[AltPartRole]int, len(n.Parts))
	for _, part := range n.Parts {
		if !allowedPart[part.Role] {
			return fmt.Errorf("%w: %s kind %q has part role %q",
				ErrInvalidAltDescription, path, n.Kind, part.Role)
		}
		counts[part.Role]++
	}
	for _, required := range []struct {
		role AltPartRole
		want int
	}{
		{AltPartNumerator, requireParts[AltPartNumerator]},
		{AltPartDenominator, requireParts[AltPartDenominator]},
		{AltPartRadicand, requireParts[AltPartRadicand]},
		{AltPartBase, requireParts[AltPartBase]},
		{AltPartBody, requireParts[AltPartBody]},
		{AltPartAccent, requireParts[AltPartAccent]},
	} {
		if required.want != 0 && counts[required.role] != required.want {
			return fmt.Errorf("%w: %s kind %q has %d %q parts, want %d",
				ErrInvalidAltDescription, path, n.Kind, counts[required.role],
				required.role, required.want)
		}
	}
	for _, part := range n.Parts {
		if part.Role != AltPartContent && part.Role != AltPartItem && counts[part.Role] > 1 {
			return fmt.Errorf("%w: %s kind %q has duplicate part role %q",
				ErrInvalidAltDescription, path, n.Kind, part.Role)
		}
	}
	return nil
}

func validateAltText(text, path string) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidAltDescription, path)
	}
	if strings.ContainsRune(text, '\x00') {
		return fmt.Errorf("%w: %s contains NUL", ErrInvalidAltDescription, path)
	}
	return nil
}

func (k AltNodeKind) valid() bool {
	switch k {
	case AltKindRow, AltKindGlyph, AltKindText, AltKindOperator, AltKindSpace,
		AltKindFraction, AltKindRadical, AltKindScripts, AltKindFenced,
		AltKindSizedDelimiter, AltKindAccent, AltKindLine, AltKindCancel,
		AltKindTable, AltKindMultiline, AltKindStyle, AltKindOverlay, AltKindRaise,
		AltKindColor, AltKindBox, AltKindPhantom, AltKindError,
		AltKindPlaceholder, AltKindBreak, AltKindAlignMarker:
		return true
	default:
		return false
	}
}

func (r AltPartRole) valid() bool {
	switch r {
	case AltPartItem, AltPartBody, AltPartNumerator, AltPartDenominator,
		AltPartRadicand, AltPartIndex, AltPartBase, AltPartTop, AltPartBottom,
		AltPartTopLeft, AltPartBottomLeft, AltPartTopRight, AltPartBottomRight,
		AltPartOpen, AltPartClose, AltPartContent, AltPartAccent:
		return true
	default:
		return false
	}
}

func (v AltVariant) valid() bool {
	switch v {
	case AltVariantNormal, AltVariantItalic, AltVariantBold, AltVariantBoldItalic,
		AltVariantDoubleStruck, AltVariantScript, AltVariantBoldScript,
		AltVariantFraktur, AltVariantBoldFraktur, AltVariantSans,
		AltVariantBoldSans, AltVariantMonospace:
		return true
	default:
		return false
	}
}

func (r AltFractionRole) valid() bool {
	return r == AltFractionOrdinary || r == AltFractionBinomial || r == AltFractionStacked
}

func (s AltFractionStyle) valid() bool {
	return s == AltFractionVertical || s == AltFractionNoBar ||
		s == AltFractionSkewed || s == AltFractionHorizontal
}

func (r AltScriptRole) valid() bool {
	return r == AltScriptOrdinary || r == AltScriptLimits ||
		r == AltScriptOverlay || r == AltScriptBraceLabel
}

func (r AltTableRole) valid() bool {
	return r == AltTableMatrix || r == AltTableArray ||
		r == AltTableCases || r == AltTableVector
}

func (r AltCancelRole) valid() bool {
	return r == AltCancelStrike || r == AltCancelNegation || r == AltCancelSlashed
}

func (p AltPosition) valid() bool {
	return p == AltPositionAbove || p == AltPositionBelow
}

func (s AltSpaceKind) valid() bool {
	return s == AltSpaceExplicit || s == AltSpaceInterword || s == AltSpaceIgnored
}

func (w AltSpaceWidth) valid() bool {
	switch w {
	case AltSpaceWidthZero, AltSpaceWidthThin, AltSpaceWidthMedium,
		AltSpaceWidthThick, AltSpaceWidthEn, AltSpaceWidthQuad,
		AltSpaceWidthDoubleQuad, AltSpaceWidthNegativeThin,
		AltSpaceWidthNegativeMedium, AltSpaceWidthNegativeThick,
		AltSpaceWidthCustom:
		return true
	default:
		return false
	}
}

func (b AltBreakKind) valid() bool {
	return b == AltBreakRelation || b == AltBreakBinary || b == AltBreakExplicit
}

func (s AltSizingKind) valid() bool {
	return s == AltSizingContent || s == AltSizingRelative || s == AltSizingAbsolute
}

func (a AltColumnAlign) valid() bool {
	return a == AltColumnCenter || a == AltColumnLeft || a == AltColumnRight
}

func (s AltMathSize) valid() bool {
	return s == AltSizeInherit || s == AltSizeDisplay || s == AltSizeText ||
		s == AltSizeScript || s == AltSizeScriptScript
}

func (a AltOverlayAlignment) valid() bool {
	return a == AltOverlayStart || a == AltOverlayCenter || a == AltOverlayEnd
}

func (r AltRaiseDirection) valid() bool {
	return r == AltRaiseUp || r == AltRaiseDown || r == AltRaiseZero
}

func (f AltFlag) valid() bool {
	switch f {
	case AltFlagBalanced, AltFlagBar, AltFlagWeak, AltFlagStretchy, AltFlagNaturalFit,
		AltFlagDotless, AltFlagExactFrameWidth, AltFlagCross, AltFlagInverted,
		AltFlagCentered, AltFlagCramped, AltFlagUncramped, AltFlagFill,
		AltFlagBorder, AltFlagPhantomWidth, AltFlagPhantomHeight,
		AltFlagPhantomDepth:
		return true
	default:
		return false
	}
}
