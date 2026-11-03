package display

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/geometry"
)

var ErrInvalidBox = errors.New("display: invalid box")

// ErrInvalidAlt reports malformed alternative text.
var ErrInvalidAlt = errors.New("display: invalid alternative text")

// Validate checks structural and numeric output invariants.
func (b *Box) Validate() error {
	if b == nil {
		return fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	for _, field := range []struct {
		name        string
		value       float32
		nonnegative bool
	}{
		{"width", b.Width, true}, {"ascent", b.Ascent, true}, {"descent", b.Descent, true},
		{"italic", b.Italic, false}, {"axis-height", b.AxisHeight, true}, {"overflow", b.Overflow, true},
	} {
		if !finite(field.value) || field.nonnegative && field.value < 0 {
			return fmt.Errorf("%w: %s is %v", ErrInvalidBox, field.name, field.value)
		}
	}
	for i, path := range b.Paths {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("%w: path %d: %v", ErrInvalidBox, i, err)
		}
	}
	for i, item := range b.Items {
		if err := validateItem(item, b.Paths); err != nil {
			return fmt.Errorf("%w: item %d: %v", ErrInvalidBox, i, err)
		}
	}
	for i, hit := range b.Hits {
		if err := validateHit(hit); err != nil {
			return fmt.Errorf("%w: hit %d: %v", ErrInvalidBox, i, err)
		}
	}
	for i, breakPoint := range b.Breaks {
		if err := validateBreak(breakPoint); err != nil {
			return fmt.Errorf("%w: break %d: %v", ErrInvalidBox, i, err)
		}
	}
	if err := b.Alt.Validate(); err != nil {
		return fmt.Errorf("%w: alt: %v", ErrInvalidBox, err)
	}
	if b.Tag != nil {
		if err := validateTagBox(b.Tag, len(b.Items), len(b.Hits)); err != nil {
			return fmt.Errorf("%w: tag: %v", ErrInvalidBox, err)
		}
		if !closeCoordinate(b.Italic, b.Tag.Italic) {
			return fmt.Errorf("%w: parent italic %v, want rightmost tag run %v", ErrInvalidBox, b.Italic, b.Tag.Italic)
		}
	}
	if len(b.Lines) > 0 {
		if err := validateLines(b); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBox, err)
		}
	} else if b.Tag != nil {
		if err := validateUnwrappedTagAggregate(b); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBox, err)
		}
	}
	return nil
}

func validateLines(box *Box) error {
	itemLimit := len(box.Items)
	hitLimit := len(box.Hits)
	if box.Tag != nil {
		itemLimit = box.Tag.Items.Start
		hitLimit = box.Tag.Hits.Start
	}

	itemOffset := 0
	hitOffset := 0
	lineTagCount := 0
	for i, line := range box.Lines {
		if err := validateLineMetrics(line, i, box); err != nil {
			return err
		}
		if err := validateItemRange(line.Items, len(box.Items), fmt.Sprintf("line %d items", i)); err != nil {
			return err
		}
		if err := validateHitRange(line.Hits, len(box.Hits), fmt.Sprintf("line %d hits", i)); err != nil {
			return err
		}
		if line.Items.Start != itemOffset {
			return fmt.Errorf("line %d item range starts at %d, want %d", i, line.Items.Start, itemOffset)
		}
		if line.Hits.Start != hitOffset {
			return fmt.Errorf("line %d hit range starts at %d, want %d", i, line.Hits.Start, hitOffset)
		}
		if i > 0 {
			previous := box.Lines[i-1]
			want := previous.Baseline + previous.Descent + line.Ascent
			if absFloat(line.Baseline-want) > lineTolerance(want) {
				return fmt.Errorf("line %d baseline %v, want %v", i, line.Baseline, want)
			}
		}
		if err := validateLineOverflow(box.Items, line, box.Width, i); err != nil {
			return err
		}
		if line.Tag != nil {
			lineTagCount++
			if box.Tag == nil {
				return fmt.Errorf("line %d has a tag but parent tag is nil", i)
			}
			if i != 0 {
				return fmt.Errorf("line %d has a tag outside the phase-1 target line", i)
			}
			if line.Tag != box.Tag {
				return fmt.Errorf("line %d tag must reference the canonical parent tag", i)
			}
		}
		itemOffset = line.Items.End()
		hitOffset = line.Hits.End()
	}
	if itemOffset != itemLimit {
		return fmt.Errorf("line item ranges cover %d records, want %d before tag", itemOffset, itemLimit)
	}
	if hitOffset != hitLimit {
		return fmt.Errorf("line hit ranges cover %d records, want %d before tag", hitOffset, hitLimit)
	}
	if lineTagCount > 1 {
		return fmt.Errorf("line tag count %d exceeds single-line phase-1 contract", lineTagCount)
	}
	if box.Tag != nil && box.Lines[0].Tag == nil {
		return fmt.Errorf("parent tag targets line 0 without matching line tag")
	}
	if box.Tag != nil {
		if err := validateTagRangeSuffix(box.Tag, len(box.Items), len(box.Hits)); err != nil {
			return err
		}
	}
	return validateWrappedOverflow(box)
}

func validateLineMetrics(line LineBox, index int, box *Box) error {
	for _, field := range []struct {
		name  string
		value float32
	}{
		{"baseline", line.Baseline},
		{"offset-x", line.OffsetX},
		{"width", line.Width},
		{"ascent", line.Ascent},
		{"descent", line.Descent},
		{"overflow", line.Overflow},
	} {
		if !finite(field.value) || field.value < 0 {
			return fmt.Errorf("line %d %s is %v", index, field.name, field.value)
		}
	}
	if line.Width > box.Width+lineTolerance(box.Width) {
		if line.OffsetX != 0 || line.Overflow <= 0 {
			return fmt.Errorf("line %d overflows parent frame without a zero offset and overflow", index)
		}
	} else if line.OffsetX+line.Width > box.Width+lineTolerance(box.Width) {
		return fmt.Errorf("line %d exceeds parent frame", index)
	}
	top := line.Baseline - line.Ascent
	bottom := line.Baseline + line.Descent
	if top < -box.Ascent-lineTolerance(box.Ascent) ||
		bottom > box.Descent+lineTolerance(box.Descent) {
		return fmt.Errorf("line %d extent [%v,%v] exceeds parent [%v,%v]",
			index, top, bottom, -box.Ascent, box.Descent)
	}
	return nil
}

func validateLineOverflow(items []Item, line LineBox, parentWidth float32, index int) error {
	extent, err := inkExtentForRange(items, line.Items, line.OffsetX, line.Baseline)
	if err != nil {
		return fmt.Errorf("line %d overflow scan: %v", index, err)
	}
	ink := extent.overflow(line.Width, line.Ascent, line.Descent)
	logical := maxFloat(0, line.Width-parentWidth)
	required := maxFloat(ink, logical)
	if !overflowAtLeast(line.Overflow, required) {
		return fmt.Errorf("line %d overflow %v is below required %v", index, line.Overflow, required)
	}
	return nil
}

func validateTagBox(tag *TagBox, itemCount, hitCount int) error {
	if !tag.Source.IsValid() || tag.Source.IsDetached() {
		return errors.New("source range is invalid")
	}
	if tag.Line != FormulaTagLine {
		return fmt.Errorf("line is %d, want formula-level %d", tag.Line, FormulaTagLine)
	}
	if !tag.Manual {
		return errors.New("tag must be manual in the Phase-1 contract")
	}
	for _, field := range []struct {
		name  string
		value float32
	}{
		{"x", tag.X},
		{"y", tag.Y},
		{"width", tag.Width},
		{"ascent", tag.Ascent},
		{"descent", tag.Descent},
	} {
		if !finite(field.value) {
			return fmt.Errorf("%s is %v", field.name, field.value)
		}
	}
	if tag.X < 0 || tag.Width < 0 || tag.Ascent < 0 || tag.Descent < 0 {
		return errors.New("tag geometry contains a negative size")
	}
	if !finite(tag.Italic) {
		return fmt.Errorf("italic is %v", tag.Italic)
	}
	if err := validateItemRange(tag.Items, itemCount, "items"); err != nil {
		return err
	}
	if err := validateHitRange(tag.Hits, hitCount, "hits"); err != nil {
		return err
	}
	return nil
}

func validateItemRange(r ItemRange, length int, path string) error {
	if r.Start < 0 || r.Count < 0 || r.Start > length || r.Count > length-r.Start {
		return fmt.Errorf("%s range [%d,%d) exceeds length %d", path, r.Start, r.End(), length)
	}
	return nil
}

func validateHitRange(r HitRange, length int, path string) error {
	if r.Start < 0 || r.Count < 0 || r.Start > length || r.Count > length-r.Start {
		return fmt.Errorf("%s range [%d,%d) exceeds length %d", path, r.Start, r.End(), length)
	}
	return nil
}

func validateTagRangeSuffix(tag *TagBox, itemCount, hitCount int) error {
	if tag.Items.End() != itemCount {
		return fmt.Errorf("tag item range ends at %d, want parent length %d", tag.Items.End(), itemCount)
	}
	if tag.Hits.End() != hitCount {
		return fmt.Errorf("tag hit range ends at %d, want parent length %d", tag.Hits.End(), hitCount)
	}
	return nil
}

func validateUnwrappedTagAggregate(box *Box) error {
	if err := validateTagRangeSuffix(box.Tag, len(box.Items), len(box.Hits)); err != nil {
		return err
	}
	frame := inkExtentForItems(box.Items)
	required := frame.overflow(box.Width, box.Ascent, box.Descent)
	required = maxFloat(required, maxFloat(0, box.Tag.X+box.Tag.Width-box.Width))
	if !overflowAtLeast(box.Overflow, required) {
		return fmt.Errorf("parent overflow %v is below tag requirement %v", box.Overflow, required)
	}
	return nil
}

func validateWrappedOverflow(box *Box) error {
	frame := inkExtentForItems(box.Items)
	required := frame.overflow(box.Width, box.Ascent, box.Descent)
	for _, line := range box.Lines {
		required = maxFloat(required, maxFloat(0, line.Width-box.Width))
	}
	if box.Tag != nil {
		required = maxFloat(required, maxFloat(0, box.Tag.X+box.Tag.Width-box.Width))
	}
	if !overflowAtLeast(box.Overflow, required) {
		return fmt.Errorf("parent overflow %v is below required %v", box.Overflow, required)
	}
	return nil
}

type inkExtent struct {
	minX float32
	minY float32
	maxX float32
	maxY float32
	have bool
}

func inkExtentForItems(items []Item) inkExtent {
	var extent inkExtent
	for _, item := range items {
		extent.addRect(item.Bound)
	}
	return extent
}

func inkExtentForRange(items []Item, itemRange ItemRange, dx, dy float32) (inkExtent, error) {
	var extent inkExtent
	for i := itemRange.Start; i < itemRange.End(); i++ {
		item := items[i]
		if err := extent.addTranslatedRect(item.Bound, dx, dy); err != nil {
			return inkExtent{}, err
		}
	}
	return extent, nil
}

func (e *inkExtent) addRect(rect geometry.Rect) {
	e.addPoint(geometry.Point{X: rect.X, Y: rect.Y})
	e.addPoint(geometry.Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height})
}

func (e *inkExtent) addTranslatedRect(rect geometry.Rect, dx, dy float32) error {
	if err := e.addTranslatedPoint(geometry.Point{X: rect.X, Y: rect.Y}, dx, dy); err != nil {
		return err
	}
	return e.addTranslatedPoint(
		geometry.Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height},
		dx,
		dy,
	)
}

func (e *inkExtent) addTranslatedPoint(point geometry.Point, dx, dy float32) error {
	x := float64(point.X) - float64(dx)
	y := float64(point.Y) - float64(dy)
	if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) ||
		math.Abs(x) > math.MaxFloat32 || math.Abs(y) > math.MaxFloat32 {
		return errors.New("local coordinate overflow")
	}
	e.addPoint(geometry.Point{X: float32(x), Y: float32(y)})
	return nil
}

func (e *inkExtent) addPoint(point geometry.Point) {
	if !e.have {
		e.minX, e.maxX = point.X, point.X
		e.minY, e.maxY = point.Y, point.Y
		e.have = true
		return
	}
	e.minX = minFloat(e.minX, point.X)
	e.minY = minFloat(e.minY, point.Y)
	e.maxX = maxFloat(e.maxX, point.X)
	e.maxY = maxFloat(e.maxY, point.Y)
}

func (e inkExtent) overflow(width, ascent, descent float32) float32 {
	if !e.have {
		return 0
	}
	left := maxFloat(0, -e.minX)
	right := maxFloat(0, e.maxX-width)
	top := maxFloat(0, -e.minY-ascent)
	bottom := maxFloat(0, e.maxY-descent)
	return maxFloat(maxFloat(left, right), maxFloat(top, bottom))
}

func overflowAtLeast(actual, required float32) bool {
	return actual+lineTolerance(required) >= required
}

func closeCoordinate(got, want float32) bool {
	return absFloat(got-want) <= lineTolerance(want)
}

func lineTolerance(value float32) float32 {
	return maxFloat(1e-5, absFloat(value)*1e-6)
}

// Validate checks that alternative text is valid UTF-8 and contains no NUL.
func (a Alt) Validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"speech", a.Speech},
		{"text", a.Text},
	} {
		if !utf8.ValidString(field.value) {
			return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidAlt, field.name)
		}
		if strings.ContainsRune(field.value, '\x00') {
			return fmt.Errorf("%w: %s contains NUL", ErrInvalidAlt, field.name)
		}
	}
	return nil
}

func validateHit(hit Hit) error {
	if !hit.Source.IsValid() {
		return errors.New("invalid source range")
	}
	if err := validateRect(hit.Area); err != nil {
		return fmt.Errorf("area: %v", err)
	}
	if hit.Depth < 0 {
		return errors.New("negative depth")
	}
	if hit.Kind > HitOther {
		return fmt.Errorf("unknown kind %d", hit.Kind)
	}
	return nil
}

func validateBreak(breakPoint Break) error {
	if !finite(breakPoint.X) || breakPoint.X < 0 {
		return errors.New("invalid x position")
	}
	if breakPoint.Kind > BreakExplicit {
		return fmt.Errorf("unknown kind %d", breakPoint.Kind)
	}
	return nil
}

func validateItem(item Item, paths []PathTemplate) error {
	if item.Kind > ItemGlyphRef {
		return fmt.Errorf("unknown kind %d", item.Kind)
	}
	if err := validatePaint(item.Paint); err != nil {
		return fmt.Errorf("paint: %v", err)
	}
	if err := validateRect(item.Bound); err != nil {
		return fmt.Errorf("bound: %v", err)
	}
	switch item.Kind {
	case ItemGlyphPath:
		if item.PathIndex < 0 || item.PathIndex >= len(paths) {
			return fmt.Errorf("path index %d exceeds path count %d", item.PathIndex, len(paths))
		}
		path := paths[item.PathIndex]
		if len(path.Segments) == 0 {
			return errors.New("empty glyph path")
		}
		if !finite(item.At.X) || !finite(item.At.Y) || !finite(item.Em) || item.Em <= 0 {
			return errors.New("glyph path has invalid placement")
		}
		placed := geometry.Rect{
			X:      item.At.X + path.Bound.X*item.Em,
			Y:      item.At.Y + path.Bound.Y*item.Em,
			Width:  path.Bound.Width * item.Em,
			Height: path.Bound.Height * item.Em,
		}
		if !finite(placed.X) || !finite(placed.Y) || !finite(placed.Width) || !finite(placed.Height) {
			return errors.New("glyph path placement overflows")
		}
		if !rectContains(item.Bound, placed) {
			return errors.New("glyph path bound does not cover its placed template")
		}
	case ItemRule:
		if item.PathIndex != NoPath {
			return errors.New("rule has a path index")
		}
		return validateRect(item.Rect)
	case ItemGlyphRef:
		if item.PathIndex != NoPath {
			return errors.New("glyph ref has a path index")
		}
		if item.FontID == "" {
			return errors.New("glyph ref has empty font ID")
		}
		if !finite(item.At.X) || !finite(item.At.Y) || !finite(item.Em) || item.Em <= 0 {
			return errors.New("glyph ref has invalid placement")
		}
	}
	return nil
}

func validatePath(path PathTemplate) error {
	if len(path.Segments) == 0 {
		return errors.New("empty path")
	}
	if err := validateRect(path.Bound); err != nil {
		return fmt.Errorf("bound: %v", err)
	}
	leftTolerance := lineTolerance(path.Bound.X)
	topTolerance := lineTolerance(path.Bound.Y)
	right := path.Bound.X + path.Bound.Width
	bottom := path.Bound.Y + path.Bound.Height
	rightTolerance := lineTolerance(right)
	bottomTolerance := lineTolerance(bottom)
	for i, seg := range path.Segments {
		points, ok := segmentPointCount(seg.Op)
		if !ok {
			return fmt.Errorf("segment %d has unknown op %d", i, seg.Op)
		}
		for j := 0; j < points; j++ {
			if !finite(seg.Points[j].X) || !finite(seg.Points[j].Y) {
				return fmt.Errorf("segment %d has non-finite point", i)
			}
			point := seg.Points[j]
			if point.X < path.Bound.X-leftTolerance ||
				point.Y < path.Bound.Y-topTolerance ||
				point.X > right+rightTolerance ||
				point.Y > bottom+bottomTolerance {
				return fmt.Errorf("segment %d point %d exceeds bound", i, j)
			}
		}
	}
	return nil
}

func rectContains(outer, inner geometry.Rect) bool {
	return inner.X >= outer.X-lineTolerance(inner.X) &&
		inner.Y >= outer.Y-lineTolerance(inner.Y) &&
		inner.X+inner.Width <= outer.X+outer.Width+lineTolerance(inner.X+inner.Width) &&
		inner.Y+inner.Height <= outer.Y+outer.Height+lineTolerance(inner.Y+inner.Height)
}

func segmentPointCount(op geometry.SegOp) (int, bool) {
	if op > geometry.SegClose {
		return 0, false
	}
	return [...]int{1, 1, 2, 3, 0}[op], true
}

func validatePaint(paint Paint) error {
	if paint.Role > RolePlaceholder {
		return fmt.Errorf("unknown role %d", paint.Role)
	}
	if paint.HasExplicit && paint.Role != RoleInk {
		return fmt.Errorf("explicit color has non-ink role %d", paint.Role)
	}
	return nil
}

func validateRect(r geometry.Rect) error {
	for _, value := range []float32{r.X, r.Y, r.Width, r.Height} {
		if !finite(value) {
			return errors.New("non-finite value")
		}
	}
	if r.Width < 0 || r.Height < 0 {
		return errors.New("negative size")
	}
	return nil
}

func finite(v float32) bool {
	value := float64(v)
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
