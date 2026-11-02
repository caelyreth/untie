package layout

import (
	"bytes"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/source"
)

func FuzzDisplayLineAlignment(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 10, 4})
	f.Add([]byte{1, 20, 8, 4, 0, 12, 3, 1})
	f.Add([]byte{2, 255, 255, 255, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		cursor := fuzzByteCursor{data: data}
		align := display.Align(cursor.next() % 3)
		maxWidth := float32(cursor.next()%64 + 1)
		lineCount := int(cursor.next() % 9)
		lines := make([]display.LineBox, lineCount)
		for i := range lines {
			width := float32(cursor.next() % 80)
			ascent := float32(cursor.next()%24 + 1)
			descent := float32(cursor.next() % 12)
			localX := float32(int8(cursor.next())) / 8
			overflow := maxFloat(0, width-maxWidth)
			overflow = maxFloat(overflow, maxFloat(0, -localX))
			overflow = maxFloat(overflow, maxFloat(0, localX))
			lines[i] = display.LineBox{Box: &display.Box{
				Width:    width,
				Ascent:   ascent,
				Descent:  descent,
				Overflow: overflow,
				Items: []display.Item{{
					Kind:  display.ItemRule,
					Bound: geometry.Rect{X: localX, Y: -ascent, Width: width, Height: 1},
					Rect:  geometry.Rect{X: localX, Y: -ascent, Width: width, Height: 1},
				}},
				Hits: []display.Hit{{
					Source: source.NewRange(0, 1),
					Area:   geometry.Rect{X: localX, Y: -ascent, Width: width, Height: 1},
					Kind:   display.HitAtom,
				}},
			}}
		}
		state := alignmentLayoutState(align, maxWidth)
		box, err := state.assembleDisplayLines(&display.Box{}, lines)
		if lineCount == 0 {
			if err == nil {
				t.Fatal("empty input assembled a box")
			}
			return
		}
		if err != nil {
			t.Fatalf("assembleDisplayLines() error = %v", err)
		}
		if err := box.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if box.Width != maxWidth {
			t.Fatalf("Width = %v, want frame %v", box.Width, maxWidth)
		}
		if len(box.Items) != lineCount || len(box.Hits) != lineCount {
			t.Fatalf("aggregate = %d items/%d hits, want %d", len(box.Items), len(box.Hits), lineCount)
		}
		for i, line := range box.Lines {
			local := lines[i].Box
			wantOffset := float32(0)
			if local.Width < maxWidth {
				switch align {
				case display.AlignCenter:
					wantOffset = (maxWidth - local.Width) / 2
				case display.AlignEnd:
					wantOffset = maxWidth - local.Width
				}
			}
			if line.OffsetX != wantOffset {
				t.Fatalf("line %d OffsetX = %v, want %v", i, line.OffsetX, wantOffset)
			}
			if line.Box.Items[0].Bound.X != local.Items[0].Bound.X {
				t.Fatalf("line %d local item x changed", i)
			}
			wantX := local.Items[0].Bound.X + wantOffset
			if !closeHost(box.Items[i].Bound.X, wantX) {
				t.Fatalf("line %d aggregate x = %v, want %v", i, box.Items[i].Bound.X, wantX)
			}
			if !closeHost(box.Hits[i].Area.X, wantX) {
				t.Fatalf("line %d aggregate hit x = %v, want %v", i, box.Hits[i].Area.X, wantX)
			}
		}
		first, err := box.MarshalCanonical()
		if err != nil {
			t.Fatalf("MarshalCanonical() error = %v", err)
		}
		second, err := box.Clone().MarshalCanonical()
		if err != nil {
			t.Fatalf("clone MarshalCanonical() error = %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatal("clone changed canonical output")
		}
	})
}

type fuzzByteCursor struct {
	data []byte
}

func (c *fuzzByteCursor) next() byte {
	if len(c.data) == 0 {
		return 0
	}
	value := c.data[0]
	c.data = c.data[1:]
	return value
}
