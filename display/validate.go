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
	if len(b.Lines) > 0 {
		if !allowLines {
			return fmt.Errorf("%w: nested lines", ErrInvalidBox)
		}
		if err := validateLines(b); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBox, err)
		}
	}
	return nil
}

func validateLines(box *Box) error {
	itemCount := 0
	hitCount := 0
	for i, line := range box.Lines {
		if !finite(line.Baseline) || line.Baseline < 0 {
			return fmt.Errorf("line %d baseline is %v", i, line.Baseline)
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
		if line.Box.Width > box.Width+lineTolerance(box.Width) {
			return fmt.Errorf("line %d width %v exceeds parent width %v", i, line.Box.Width, box.Width)
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
	if len(box.Items) != itemCount {
		return fmt.Errorf("parent item count %d, want line aggregate %d", len(box.Items), itemCount)
	}
	if len(box.Hits) != hitCount {
		return fmt.Errorf("parent hit count %d, want line aggregate %d", len(box.Hits), hitCount)
	}
	return nil
}

func lineTolerance(value float32) float32 {
	return maxFloat(1e-5, absFloat(value)*1e-6)
}

// Validate checks that alternative text is valid UTF-8 and that deferred
// output remains empty.
func (a Alt) Validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"speech", a.Speech},
		{"text", a.Text},
		{"mathml", a.MathML},
	} {
		if !utf8.ValidString(field.value) {
			return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidAlt, field.name)
		}
		if strings.ContainsRune(field.value, '\x00') {
			return fmt.Errorf("%w: %s contains NUL", ErrInvalidAlt, field.name)
		}
	}
	if a.MathML != "" {
		return fmt.Errorf("%w: MathML is not implemented", ErrInvalidAlt)
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
			if seg.Op > geometry.SegClose {
				return fmt.Errorf("segment %d has unknown op %d", i, seg.Op)
			}
			points := 1
			switch seg.Op {
			case geometry.SegQuadTo:
				points = 2
			case geometry.SegCubeTo:
				points = 3
			case geometry.SegClose:
				points = 0
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
