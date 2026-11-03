package svg_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/encode/svg"
	"github.com/caelyreth/untie/geometry"
)

func TestMarshalGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		box     *display.Box
		options *svg.Options
	}{
		{name: "mixed", box: mixedBox(), options: mixedOptions()},
		{name: "empty", box: &display.Box{}, options: &svg.Options{Padding: 2}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			before := test.box.Clone()
			got, err := svg.Marshal(test.box, test.options)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", test.name+".svg"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("Marshal() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
			}
			if !reflect.DeepEqual(test.box, before) {
				t.Fatal("Marshal() mutated the box")
			}
		})
	}
}

func TestWriteMatchesMarshal(t *testing.T) {
	t.Parallel()

	box := mixedBox()
	want, err := svg.Marshal(box, mixedOptions())
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got bytes.Buffer
	if err := svg.Write(&got, box, mixedOptions()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("Write() differs from Marshal()\n--- Write ---\n%s--- Marshal ---\n%s", got.Bytes(), want)
	}
}

func TestMarshalIsDeterministicAndXMLValid(t *testing.T) {
	t.Parallel()

	first, err := svg.Marshal(mixedBox(), mixedOptions())
	if err != nil {
		t.Fatalf("first Marshal() error = %v", err)
	}
	second, err := svg.Marshal(mixedBox(), mixedOptions())
	if err != nil {
		t.Fatalf("second Marshal() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Marshal() output is not deterministic")
	}

	var document struct {
		XMLName xml.Name `xml:"svg"`
		Width   string   `xml:"width,attr"`
		Height  string   `xml:"height,attr"`
		ViewBox string   `xml:"viewBox,attr"`
		Title   string   `xml:"title"`
		Desc    string   `xml:"desc"`
	}
	if err := xml.Unmarshal(first, &document); err != nil {
		t.Fatalf("XML is invalid: %v\n%s", err, first)
	}
	if document.XMLName.Space != "http://www.w3.org/2000/svg" {
		t.Fatalf("namespace = %q", document.XMLName.Space)
	}
	if document.Width != "16" || document.Height != "12" || document.ViewBox != "0 0 16 12" {
		t.Fatalf("document geometry = width %q height %q viewBox %q",
			document.Width, document.Height, document.ViewBox)
	}
	if document.Title != `<Formula & "quoted">` {
		t.Fatalf("title = %q", document.Title)
	}
	if document.Desc != `A < B & C` {
		t.Fatalf("description = %q", document.Desc)
	}
}

func TestMarshalUsesPaletteAndExplicitPaint(t *testing.T) {
	t.Parallel()

	options := &svg.Options{
		Padding:    1,
		Precision:  2,
		Background: &color.NRGBA{R: 0x01, G: 0x02, B: 0x03, A: 0x80},
		Palette: svg.Palette{
			Ink:         color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff},
			Error:       color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff},
			Placeholder: color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 0xff},
		},
	}
	output, err := svg.Marshal(mixedBox(), options)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := []string{
		"#112233",
		"rgba(1,2,3,0.5)",
		"#445566",
		"#778899",
		"rgba(17,34,51,0.5)",
	}
	got := fillAttributes(t, output)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fill order = %q, want %q\n%s", got, want, output)
	}
}

func fillAttributes(t *testing.T, document []byte) []string {
	t.Helper()

	decoder := xml.NewDecoder(bytes.NewReader(document))
	var fills []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode XML: %v", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attribute := range start.Attr {
			if attribute.Name.Local == "fill" {
				fills = append(fills, attribute.Value)
			}
		}
	}
	return fills
}

func TestMarshalDefaultsAndTransparentBackground(t *testing.T) {
	t.Parallel()

	output, err := svg.Marshal(mixedBox(), nil)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	text := string(output)
	if strings.Contains(text, `<rect x="0" y="0"`) {
		t.Fatalf("nil background emitted a background rectangle:\n%s", text)
	}
	if !strings.Contains(text, `fill="#000000"`) {
		t.Fatalf("default ink palette was not used:\n%s", text)
	}
}

func TestMarshalRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	invalidPath := mixedBox()
	invalidPath.Items[0].PathIndex = len(invalidPath.Paths)
	invalidPathStart := mixedBox()
	invalidPathStart.Paths[0].Segments[0].Op = geometry.SegLineTo
	tests := []struct {
		name    string
		box     *display.Box
		options *svg.Options
		want    error
	}{
		{name: "nil box", options: &svg.Options{}, want: svg.ErrInvalidBox},
		{name: "invalid box", box: invalidPath, options: &svg.Options{}, want: svg.ErrInvalidBox},
		{name: "path without MoveTo", box: invalidPathStart, options: &svg.Options{}, want: svg.ErrInvalidBox},
		{
			name: "negative padding",
			box:  &display.Box{},
			options: &svg.Options{
				Padding: -1,
			},
			want: svg.ErrInvalidOptions,
		},
		{
			name:    "negative precision",
			box:     &display.Box{},
			options: &svg.Options{Precision: -1},
			want:    svg.ErrInvalidOptions,
		},
		{
			name:    "excess precision",
			box:     &display.Box{},
			options: &svg.Options{Precision: 10},
			want:    svg.ErrInvalidOptions,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := svg.Marshal(test.box, test.options); !errors.Is(err, test.want) {
				t.Fatalf("Marshal() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWritePropagatesWriterErrors(t *testing.T) {
	t.Parallel()

	stop := errors.New("stop")
	writer := &failingWriter{remaining: 7, err: stop}
	err := svg.Write(writer, mixedBox(), mixedOptions())
	if !errors.Is(err, stop) {
		t.Fatalf("Write() error = %v, want %v", err, stop)
	}
}

func TestWriteRejectsNilWriter(t *testing.T) {
	t.Parallel()

	err := svg.Write(nil, mixedBox(), mixedOptions())
	if !errors.Is(err, svg.ErrInvalidWriter) {
		t.Fatalf("Write() error = %v", err)
	}
}

type failingWriter struct {
	remaining int
	err       error
}

func (w *failingWriter) Write(data []byte) (int, error) {
	if len(data) <= w.remaining {
		w.remaining -= len(data)
		return len(data), nil
	}
	written := w.remaining
	w.remaining = 0
	return written, w.err
}

func mixedBox() *display.Box {
	template := display.PathTemplate{
		Segments: []geometry.Segment{
			{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 0.25, Y: 0.125}}},
			{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 0.75, Y: 0.125}}},
			{Op: geometry.SegQuadTo, Points: [3]geometry.Point{{X: 0.25, Y: 0.75}, {X: 0.75, Y: 0.75}}},
			{Op: geometry.SegCubeTo, Points: [3]geometry.Point{
				{X: 0.125, Y: 0.25},
				{X: 0.875, Y: 0.25},
				{X: 0.5, Y: 0.625},
			}},
			{Op: geometry.SegClose},
		},
		Bound: geometry.Rect{X: 0.125, Y: 0.125, Width: 0.75, Height: 0.625},
	}
	return &display.Box{
		Width:      10,
		Ascent:     4,
		Descent:    2,
		Overflow:   1,
		AxisHeight: 2.5,
		Paths:      []display.PathTemplate{template, template, template, template},
		Items: []display.Item{
			pathItem(display.RoleInk, 0, geometry.Point{X: 1, Y: 2}, 2),
			{
				Kind:      display.ItemRule,
				Paint:     display.Paint{Role: display.RoleError},
				Bound:     geometry.Rect{X: 4, Y: -3, Width: 5, Height: 0.5},
				PathIndex: display.NoPath,
				Rect:      geometry.Rect{X: 4, Y: -3, Width: 5, Height: 0.5},
			},
			pathItem(display.RolePlaceholder, 1, geometry.Point{X: 3}, 1),
			{
				Kind: display.ItemGlyphPath,
				Paint: display.Paint{
					Role:        display.RoleInk,
					Explicit:    display.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x80},
					HasExplicit: true,
				},
				Bound:     geometry.Rect{X: 5.125, Y: 1.125, Width: 0.75, Height: 0.625},
				PathIndex: 2,
				At:        geometry.Point{X: 5, Y: 1},
				Em:        1,
			},
			{
				Kind: display.ItemGlyphRef, Paint: display.Paint{Role: display.RoleInk},
				Bound:     geometry.Rect{X: 7, Y: 1, Width: 1, Height: 1},
				PathIndex: display.NoPath, FontID: "math", Glyph: 7,
				At: geometry.Point{X: 7, Y: 2}, Em: 1,
			},
		},
	}
}

func pathItem(role display.Role, pathIndex int, at geometry.Point, em float32) display.Item {
	templateBound := geometry.Rect{X: 0.125, Y: 0.125, Width: 0.75, Height: 0.625}
	return display.Item{
		Kind:  display.ItemGlyphPath,
		Paint: display.Paint{Role: role},
		Bound: geometry.Rect{
			X:      at.X + templateBound.X*em,
			Y:      at.Y + templateBound.Y*em,
			Width:  templateBound.Width * em,
			Height: templateBound.Height * em,
		},
		PathIndex: pathIndex,
		At:        at,
		Em:        em,
	}
}

func mixedOptions() *svg.Options {
	background := color.NRGBA{R: 0xf8, G: 0xfa, B: 0xfc, A: 0xff}
	return &svg.Options{
		Padding:     2,
		Precision:   3,
		Background:  &background,
		Title:       `<Formula & "quoted">`,
		Description: `A < B & C`,
	}
}

var _ io.Writer = (*failingWriter)(nil)
