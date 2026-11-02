package display

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/geometry"
)

var ErrInvalidBox = errors.New("display: invalid box")

// ErrInvalidAlt reports malformed alternative text.
var ErrInvalidAlt = errors.New("display: invalid alternative text")

// Validate checks structural and numeric output invariants.
func (b *Box) Validate() error {
	return b.validate(true)
}

func (b *Box) validate(allowLines bool) error {
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
	for i, item := range b.Items {
		if err := validateItem(item); err != nil {
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
		if err := validateTagBox(b.Tag, "tag"); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBox, err)
		}
	}
	if len(b.Lines) > 0 {
		if !allowLines {
			return fmt.Errorf("%w: nested lines", ErrInvalidBox)
		}
		if err := validateLines(b); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBox, err)
		}
	}
	if b.Tag != nil && len(b.Lines) == 0 {
		if err := validateUnwrappedTagAggregate(b); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBox, err)
		}
	}
	return nil
}

func validateLines(box *Box) error {
	itemCount := 0
	hitCount := 0
	lineTagCount := 0
	for i, line := range box.Lines {
		if !finite(line.Baseline) || line.Baseline < 0 {
			return fmt.Errorf("line %d baseline is %v", i, line.Baseline)
		}
		if !finite(line.OffsetX) || line.OffsetX < 0 {
			return fmt.Errorf("line %d offset-x is %v", i, line.OffsetX)
		}
		if line.Box == nil {
			return fmt.Errorf("line %d has nil box", i)
		}
		if i > 0 {
			previous := box.Lines[i-1]
			want := previous.Baseline + previous.Box.Descent + line.Box.Ascent
			if absFloat(line.Baseline-want) > lineTolerance(want) {
				return fmt.Errorf("line %d baseline %v, want %v", i, line.Baseline, want)
			}
		}
		if err := line.Box.validate(false); err != nil {
			return fmt.Errorf("line %d: %v", i, err)
		}
		if line.Tag != nil {
			lineTagCount++
			if box.Tag == nil {
				return fmt.Errorf("line %d has a tag but parent tag is nil", i)
			}
			if line.Tag.Line >= 0 && line.Tag.Line != i {
				return fmt.Errorf("line %d tag targets line %d", i, line.Tag.Line)
			}
			if !sameTagMetadata(*line.Tag, *box.Tag) {
				return fmt.Errorf("line %d tag metadata differs from parent tag", i)
			}
		}
		if line.Box.Width > box.Width+lineTolerance(box.Width) {
			if line.OffsetX != 0 || line.Box.Overflow <= 0 {
				return fmt.Errorf("line %d overflows parent frame without a zero offset and overflow", i)
			}
		} else if line.OffsetX+line.Box.Width > box.Width+lineTolerance(box.Width) {
			return fmt.Errorf("line %d exceeds parent frame", i)
		}
		top := line.Baseline - line.Box.Ascent
		bottom := line.Baseline + line.Box.Descent
		if top < -box.Ascent-lineTolerance(box.Ascent) ||
			bottom > box.Descent+lineTolerance(box.Descent) {
			return fmt.Errorf("line %d extent [%v,%v] exceeds parent [%v,%v]", i, top, bottom, -box.Ascent, box.Descent)
		}
		itemCount += len(line.Box.Items)
		hitCount += len(line.Box.Hits)
	}
	if lineTagCount > 1 {
		return fmt.Errorf("line tag count %d exceeds single-line phase-1 contract", lineTagCount)
	}
	if box.Tag != nil && box.Tag.Line >= len(box.Lines) {
		return fmt.Errorf("parent tag targets line %d with only %d lines", box.Tag.Line, len(box.Lines))
	}
	if box.Tag != nil {
		itemCount += len(box.Tag.Box.Items)
		hitCount += len(box.Tag.Box.Hits)
	}
	if len(box.Items) != itemCount {
		return fmt.Errorf("parent item count %d, want line aggregate %d", len(box.Items), itemCount)
	}
	if len(box.Hits) != hitCount {
		return fmt.Errorf("parent hit count %d, want line aggregate %d", len(box.Hits), hitCount)
	}
	itemOffset := 0
	hitOffset := 0
	for i, line := range box.Lines {
		for j, item := range line.Box.Items {
			if err := validateTranslatedItem(box.Items[itemOffset+j], item, line.OffsetX, line.Baseline); err != nil {
				return fmt.Errorf("line %d item %d aggregate: %v", i, j, err)
			}
		}
		for j, hit := range line.Box.Hits {
			if err := validateTranslatedHit(box.Hits[hitOffset+j], hit, line.OffsetX, line.Baseline); err != nil {
				return fmt.Errorf("line %d hit %d aggregate: %v", i, j, err)
			}
		}
		itemOffset += len(line.Box.Items)
		hitOffset += len(line.Box.Hits)
	}
	if box.Tag != nil {
		for j, item := range box.Tag.Box.Items {
			if err := validateTranslatedItem(box.Items[itemOffset+j], item, box.Tag.X, box.Tag.Y); err != nil {
				return fmt.Errorf("tag item %d aggregate: %v", j, err)
			}
		}
		for j, hit := range box.Tag.Box.Hits {
			if err := validateTranslatedHit(box.Hits[hitOffset+j], hit, box.Tag.X, box.Tag.Y); err != nil {
				return fmt.Errorf("tag hit %d aggregate: %v", j, err)
			}
		}
	}
	if err := validateWrappedOverflow(box); err != nil {
		return err
	}
	return nil
}

func validateTagBox(tag *TagBox, path string) error {
	if tag.Box == nil {
		return fmt.Errorf("%s has nil box", path)
	}
	if !tag.Source.IsValid() || tag.Source.IsDetached() {
		return fmt.Errorf("%s source range is invalid", path)
	}
	if tag.Line < -1 {
		return fmt.Errorf("%s line is %d", path, tag.Line)
	}
	if !finite(tag.X) || tag.X < 0 {
		return fmt.Errorf("%s x is %v", path, tag.X)
	}
	if !finite(tag.Y) {
		return fmt.Errorf("%s y is %v", path, tag.Y)
	}
	if tag.Box.Tag != nil {
		return fmt.Errorf("%s contains a nested tag", path)
	}
	if err := tag.Box.validate(false); err != nil {
		return fmt.Errorf("%s box: %v", path, err)
	}
	return nil
}

func sameTagMetadata(left, right TagBox) bool {
	if left.Box == nil || right.Box == nil {
		return left.Box == right.Box
	}
	return left.Source == right.Source &&
		left.Manual == right.Manual &&
		left.Starred == right.Starred &&
		left.Line == right.Line &&
		left.X == right.X &&
		left.Y == right.Y &&
		reflect.DeepEqual(canonicalBoxValue(left.Box, false), canonicalBoxValue(right.Box, false))
}

func validateUnwrappedTagAggregate(box *Box) error {
	if box.Tag == nil || box.Tag.Box == nil {
		return nil
	}
	itemStart := len(box.Items) - len(box.Tag.Box.Items)
	hitStart := len(box.Hits) - len(box.Tag.Box.Hits)
	if itemStart < 0 || hitStart < 0 {
		return errors.New("tag aggregate exceeds parent output")
	}
	for i, item := range box.Tag.Box.Items {
		if err := validateTranslatedItem(box.Items[itemStart+i], item, box.Tag.X, box.Tag.Y); err != nil {
			return fmt.Errorf("tag item %d aggregate: %v", i, err)
		}
	}
	for i, hit := range box.Tag.Box.Hits {
		if err := validateTranslatedHit(box.Hits[hitStart+i], hit, box.Tag.X, box.Tag.Y); err != nil {
			return fmt.Errorf("tag hit %d aggregate: %v", i, err)
		}
	}
	translated := make([]Item, len(box.Tag.Box.Items))
	for i, item := range box.Tag.Box.Items {
		translated[i] = translateItemForValidation(item, box.Tag.X, box.Tag.Y)
	}
	required := inkExtentForItems(translated).overflow(box.Width, box.Ascent, box.Descent)
	if !overflowAtLeast(box.Overflow, required) {
		return fmt.Errorf("parent overflow %v is below tag requirement %v", box.Overflow, required)
	}
	return nil
}

func translateItemForValidation(item Item, dx, dy float32) Item {
	if len(item.Path) > 0 {
		item.Path = append([]geometry.Segment(nil), item.Path...)
	}
	item.Bound.X += dx
	item.Bound.Y += dy
	item.Rect.X += dx
	item.Rect.Y += dy
	item.At.X += dx
	item.At.Y += dy
	for i := range item.Path {
		for j := range item.Path[i].Points {
			item.Path[i].Points[j].X += dx
			item.Path[i].Points[j].Y += dy
		}
	}
	return item
}

func validateWrappedOverflow(box *Box) error {
	for i, line := range box.Lines {
		local := inkExtentForItems(line.Box.Items)
		ink := local.overflow(line.Box.Width, line.Box.Ascent, line.Box.Descent)
		logical := maxFloat(0, line.Box.Width-box.Width)
		required := maxFloat(ink, logical)
		if !overflowAtLeast(line.Box.Overflow, required) {
			return fmt.Errorf("line %d overflow %v is below required %v", i, line.Box.Overflow, required)
		}
	}

	frame := inkExtentForItems(box.Items)
	required := frame.overflow(box.Width, box.Ascent, box.Descent)
	for _, line := range box.Lines {
		required = maxFloat(required, maxFloat(0, line.Box.Width-box.Width))
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
		for _, segment := range item.Path {
			points, _ := segmentPointCount(segment.Op)
			for i := 0; i < points; i++ {
				extent.addPoint(segment.Points[i])
			}
		}
	}
	return extent
}

func (e *inkExtent) addRect(rect geometry.Rect) {
	e.addPoint(geometry.Point{X: rect.X, Y: rect.Y})
	e.addPoint(geometry.Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height})
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

func validateTranslatedItem(got, local Item, dx, dy float32) error {
	if got.Kind != local.Kind || got.Paint != local.Paint ||
		got.FontID != local.FontID || got.Glyph != local.Glyph || got.Em != local.Em {
		return errors.New("item metadata differs from line-local item")
	}
	if err := validateTranslatedRect(got.Bound, local.Bound, dx, dy); err != nil {
		return fmt.Errorf("bound: %v", err)
	}
	if err := validateTranslatedRect(got.Rect, local.Rect, dx, dy); err != nil {
		return fmt.Errorf("rect: %v", err)
	}
	if !closeCoordinate(got.At.X, local.At.X+dx) || !closeCoordinate(got.At.Y, local.At.Y+dy) {
		return errors.New("placement differs from line-local item")
	}
	if len(got.Path) != len(local.Path) {
		return errors.New("path length differs from line-local item")
	}
	for i := range local.Path {
		if got.Path[i].Op != local.Path[i].Op {
			return fmt.Errorf("path segment %d op differs from line-local item", i)
		}
		for j, point := range local.Path[i].Points {
			if !closeCoordinate(got.Path[i].Points[j].X, point.X+dx) ||
				!closeCoordinate(got.Path[i].Points[j].Y, point.Y+dy) {
				return fmt.Errorf("path segment %d point %d differs from line-local item: got (%v,%v), want (%v,%v)",
					i, j, got.Path[i].Points[j].X, got.Path[i].Points[j].Y, point.X+dx, point.Y+dy)
			}
		}
	}
	return nil
}

func validateTranslatedHit(got, local Hit, dx, dy float32) error {
	if got.Source != local.Source || got.Depth != local.Depth || got.Kind != local.Kind {
		return errors.New("hit metadata differs from line-local hit")
	}
	return validateTranslatedRect(got.Area, local.Area, dx, dy)
}

func validateTranslatedRect(got, local geometry.Rect, dx, dy float32) error {
	want := geometry.Rect{
		X:      local.X + dx,
		Y:      local.Y + dy,
		Width:  local.Width,
		Height: local.Height,
	}
	if !closeCoordinate(got.X, want.X) ||
		!closeCoordinate(got.Y, want.Y) ||
		!closeCoordinate(got.Width, want.Width) ||
		!closeCoordinate(got.Height, want.Height) {
		return fmt.Errorf("rectangle %+v differs from translated line-local rectangle %+v", got, want)
	}
	return nil
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

func validateItem(item Item) error {
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
		if len(item.Path) == 0 {
			return errors.New("empty glyph path")
		}
		for i, seg := range item.Path {
			points, ok := segmentPointCount(seg.Op)
			if !ok {
				return fmt.Errorf("segment %d has unknown op %d", i, seg.Op)
			}
			for j := 0; j < points; j++ {
				if !finite(seg.Points[j].X) || !finite(seg.Points[j].Y) {
					return fmt.Errorf("segment %d has non-finite point", i)
				}
			}
		}
	case ItemRule:
		return validateRect(item.Rect)
	case ItemGlyphRef:
		if item.FontID == "" {
			return errors.New("glyph ref has empty font ID")
		}
		if !finite(item.At.X) || !finite(item.At.Y) || !finite(item.Em) || item.Em <= 0 {
			return errors.New("glyph ref has invalid placement")
		}
	}
	return nil
}

func segmentPointCount(op geometry.SegOp) (int, bool) {
	switch op {
	case geometry.SegMoveTo, geometry.SegLineTo:
		return 1, true
	case geometry.SegQuadTo:
		return 2, true
	case geometry.SegCubeTo:
		return 3, true
	case geometry.SegClose:
		return 0, true
	default:
		return 0, false
	}
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
