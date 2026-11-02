package untie

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/internal/engine"
	"github.com/caelyreth/untie/source"
)

func TestPublicEngineLifecycle(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := Parse(`x^2+\frac{1}{2}`, TeX, Inline, ParseOptions{})
	prepared, err := NewEngine(fonts, source.Limits{MaxOutputItems: 1000})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if got := prepared.Limits(); got.MaxOutputItems != 1000 {
		t.Fatalf("Limits().MaxOutputItems = %d, want 1000", got.MaxOutputItems)
	}
	if got := prepared.FontKeySHA256(); got != fonts.KeySHA256() {
		t.Fatalf("FontKeySHA256() = %x, want %x", got, fonts.KeySHA256())
	}

	features := []string{"dtls=1", "ss01=1"}
	style := Style{Size: 16, Features: features}
	first, err := prepared.LayoutContext(context.Background(), expr, style)
	if err != nil {
		t.Fatalf("prepared LayoutContext() error = %v", err)
	}
	second, err := prepared.Layout(expr, style)
	if err != nil {
		t.Fatalf("prepared Layout() error = %v", err)
	}
	firstJSON, secondJSON := mustCanonical(t, first), mustCanonical(t, second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("repeated prepared layouts differ")
	}
	if len(first.Items) == 0 || len(second.Items) == 0 {
		t.Fatal("prepared layout returned no items")
	}
	if &first.Items[0] == &second.Items[0] {
		t.Fatal("repeated layouts share Box item storage")
	}
	if len(first.Items[0].Path) > 0 && len(second.Items[0].Path) > 0 &&
		&first.Items[0].Path[0] == &second.Items[0].Path[0] {
		t.Fatal("repeated layouts share glyph path storage")
	}
	if got := style.Features; !equalStrings(got, features) {
		t.Fatalf("LayoutContext mutated style features: %v", got)
	}

	// The Engine must not retain a style's caller-owned feature backing array.
	style.Features[0] = "bad feature"
	if _, err := prepared.Layout(expr, style); !errors.Is(err, ErrInvalidStyle) {
		t.Fatalf("invalid mutated feature error = %v, want ErrInvalidStyle", err)
	}
}

func TestSnapshotLayoutStyleOwnsFeatures(t *testing.T) {
	t.Parallel()

	features := []string{"dtls=1", "ss01=2"}
	style, err := snapshotLayoutStyle(Style{Size: 16, Features: features})
	if err != nil {
		t.Fatal(err)
	}
	features[0] = "bad feature"
	if style.Features[0] != "dtls=1" || style.Features[1] != "ss01=2" {
		t.Fatalf("snapshot aliases caller features: %v", style.Features)
	}
}

func TestLayoutCacheKeyIncludesIdentityLimitsAndVersion(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	data := testFontBytes(t)
	sameBytesDifferentID, err := LoadFonts(Fonts{Math: FontData{Bytes: data, ID: "other-id"}})
	if err != nil {
		t.Fatalf("LoadFonts(other ID) error = %v", err)
	}
	base := makeLayoutEngineCacheKey(fonts, source.Limits{}, Version())
	if got := makeLayoutEngineCacheKey(fonts, source.Limits{}, Version()); got != base {
		t.Fatal("cache key is not deterministic")
	}
	if got := makeLayoutEngineCacheKey(fonts, source.Limits{MaxOutputItems: 17}, Version()); got == base {
		t.Fatal("cache key does not include limits")
	}
	if got := makeLayoutEngineCacheKey(fonts, source.Limits{}, Version()+"-next"); got == base {
		t.Fatal("cache key does not include version")
	}
	if got := makeLayoutEngineCacheKey(sameBytesDifferentID, source.Limits{}, Version()); got == base {
		t.Fatal("cache key does not include the FontSet revision")
	}
}

func TestLayoutEngineCacheIsBoundedLRU(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	cache := newLayoutEngineCache(2)
	ctx := context.Background()
	limits := []source.Limits{
		{MaxOutputItems: 100},
		{MaxOutputItems: 101},
		{MaxOutputItems: 102},
	}
	first, err := cache.get(ctx, fonts, limits[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.get(ctx, fonts, limits[1])
	if err != nil {
		t.Fatal(err)
	}
	firstAgain, err := cache.get(ctx, fonts, limits[0])
	if err != nil {
		t.Fatal(err)
	}
	if firstAgain != first {
		t.Fatal("cache hit rebuilt an unchanged engine")
	}
	if _, err := cache.get(ctx, fonts, limits[2]); err != nil {
		t.Fatal(err)
	}
	if got := cache.length(); got != 2 {
		t.Fatalf("cache length = %d, want 2", got)
	}
	firstAgain, err = cache.get(ctx, fonts, limits[0])
	if err != nil {
		t.Fatal(err)
	}
	if firstAgain != first {
		t.Fatal("recently used engine was evicted")
	}
	secondAgain, err := cache.get(ctx, fonts, limits[1])
	if err != nil {
		t.Fatal(err)
	}
	if secondAgain == second {
		t.Fatal("least-recently-used engine was not evicted")
	}
}

func TestLayoutEngineCacheSharesConcurrentBuild(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	cache := newLayoutEngineCache(4)
	limits := source.Limits{MaxNodes: 10_000}
	const callers = 16
	results := make([]*Engine, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = cache.get(context.Background(), fonts, limits)
		}()
	}
	close(start)
	wg.Wait()
	for i := range callers {
		if errs[i] != nil {
			t.Fatalf("caller %d error = %v", i, errs[i])
		}
		if results[i] != results[0] {
			t.Fatalf("caller %d received a separately built engine", i)
		}
	}
	if got := cache.length(); got != 1 {
		t.Fatalf("cache length = %d, want 1", got)
	}
}

func TestLayoutEngineCacheCancellationWhileWaiting(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	cache := newLayoutEngineCache(2)
	started := make(chan struct{})
	release := make(chan struct{})
	cache.build = func(*FontSet, source.Limits) (*Engine, error) {
		close(started)
		<-release
		return &Engine{}, nil
	}
	buildDone := make(chan error, 1)
	go func() {
		_, err := cache.get(context.Background(), fonts, source.Limits{})
		buildDone <- err
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := cache.get(ctx, fonts, source.Limits{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting cache get error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timed-out cache wait took %v", elapsed)
	}
	close(release)
	if err := <-buildDone; err != nil {
		t.Fatalf("builder error = %v", err)
	}
}

func TestLayoutEngineCacheClearPreventsInflightInsert(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	cache := newLayoutEngineCache(2)
	started := make(chan struct{})
	release := make(chan struct{})
	cache.build = func(*FontSet, source.Limits) (*Engine, error) {
		close(started)
		<-release
		return &Engine{}, nil
	}
	built := make(chan error, 1)
	go func() {
		_, err := cache.get(context.Background(), fonts, source.Limits{})
		built <- err
	}()
	<-started
	cache.clear()
	close(release)
	if err := <-built; err != nil {
		t.Fatalf("in-flight build error = %v", err)
	}
	if got := cache.length(); got != 0 {
		t.Fatalf("cache length after clear = %d, want 0", got)
	}

	cache.build = NewEngine
	if _, err := cache.get(context.Background(), fonts, source.Limits{}); err != nil {
		t.Fatalf("post-clear build error = %v", err)
	}
	if got := cache.length(); got != 1 {
		t.Fatalf("cache length after new build = %d, want 1", got)
	}
}

func TestClearLayoutEngineCache(t *testing.T) {
	fonts := loadTestFonts(t)
	expr := Parse("x", TeX, Inline, ParseOptions{})
	if _, err := Layout(expr, fonts, Style{Size: 16}); err != nil {
		t.Fatal(err)
	}
	if got := defaultLayoutEngineCache.length(); got == 0 {
		t.Fatal("convenience layout did not populate the engine cache")
	}
	ClearLayoutEngineCache()
	if got := defaultLayoutEngineCache.length(); got != 0 {
		t.Fatalf("cache length after clear = %d, want 0", got)
	}
	if _, err := Layout(expr, fonts, Style{Size: 16}); err != nil {
		t.Fatalf("layout after clear = %v", err)
	}
}

func TestLayoutLifecycleHasNoCallOrderCoupling(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := Parse(`x^2+\frac{1}{2}`, TeX, Inline, ParseOptions{})
	fresh, err := NewEngine(fonts, expr.limits)
	if err != nil {
		t.Fatal(err)
	}
	wantBox, err := fresh.Layout(expr, Style{Size: 16})
	if err != nil {
		t.Fatal(err)
	}
	want := mustCanonical(t, wantBox)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LayoutContext(ctx, expr, fonts, Style{Size: 16}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled layout error = %v", err)
	}
	if _, err := LayoutContext(context.Background(), expr, fonts, Style{}); !errors.Is(err, ErrInvalidStyle) {
		t.Fatalf("invalid style error = %v", err)
	}
	for attempt := range 4 {
		box, err := LayoutContext(context.Background(), expr, fonts, Style{Size: 16})
		if err != nil {
			t.Fatalf("layout after prior calls %d: %v", attempt, err)
		}
		if got := mustCanonical(t, box); !bytes.Equal(got, want) {
			t.Fatalf("layout after prior calls %d differs from fresh Engine output", attempt)
		}
	}
}

func TestLayoutConcurrentSharedFontsMultipleStyles(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	prepared, err := NewEngine(fonts, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	expressions := []*Expr{
		Parse("x", TeX, Inline, ParseOptions{}),
		Parse(`x^2+\frac{1}{2}`, TeX, Inline, ParseOptions{}),
		Parse(`\sqrt{y}+\sum_{i=0}^{n}i`, TeX, Display, ParseOptions{}),
		Parse(`sqrt(y) + 1/2`, Typst, Inline, ParseOptions{}),
	}
	styles := []Style{
		{Size: 12},
		{Size: 16, Features: []string{"dtls=1"}},
		{Size: 20, Features: []string{"ss01=1", "kern=0"}},
		{Size: 24, MaxWidth: 100, Align: display.AlignCenter, ScriptLevel: 1, GlyphRefs: true},
	}
	type workload struct {
		expr  *Expr
		style Style
		want  []byte
	}
	workloads := make([]workload, 0, len(expressions)*len(styles))
	for _, expr := range expressions {
		for _, style := range styles {
			box, err := prepared.Layout(expr, style)
			if err != nil {
				t.Fatalf("baseline layout error = %v", err)
			}
			workloads = append(workloads, workload{
				expr:  expr,
				style: style,
				want:  mustCanonical(t, box),
			})
		}
	}

	const (
		workers  = 24
		attempts = 8
	)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for attempt := range attempts {
				item := workloads[(worker+attempt)%len(workloads)]
				style := item.style
				style.Features = append([]string(nil), item.style.Features...)
				var box *Box
				var err error
				if attempt%2 == 0 {
					box, err = prepared.LayoutContext(context.Background(), item.expr, style)
				} else {
					box, err = LayoutContext(context.Background(), item.expr, fonts, style)
				}
				if err != nil {
					errs <- fmt.Errorf("worker %d attempt %d: %w", worker, attempt, err)
					return
				}
				if err := box.Validate(); err != nil {
					errs <- fmt.Errorf("worker %d attempt %d: invalid box: %w", worker, attempt, err)
					return
				}
				got, err := box.MarshalCanonical()
				if err != nil {
					errs <- fmt.Errorf("worker %d attempt %d: canonicalize: %w", worker, attempt, err)
					return
				}
				if !bytes.Equal(got, item.want) {
					errs <- fmt.Errorf("worker %d attempt %d: output changed under concurrency", worker, attempt)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestLayoutRejectsInvalidStyleAndNilEngine(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := Parse("x", TeX, Inline, ParseOptions{})
	prepared, err := NewEngine(fonts, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	invalid := []Style{
		{},
		{Size: 16, Align: display.AlignEnd + 1},
		{Size: 16, ScriptLevel: -1},
		{Size: 16, Features: []string{"bad feature"}},
		{Size: 16, Features: make([]string, engine.MaxStyleFeatures+1)},
	}
	for _, style := range invalid {
		if box, err := prepared.Layout(expr, style); box != nil || !errors.Is(err, ErrInvalidStyle) {
			t.Fatalf("prepared.Layout(%+v) = %v, %v, want nil, ErrInvalidStyle", style, box, err)
		}
		if box, err := LayoutContext(context.Background(), expr, fonts, style); box != nil || !errors.Is(err, ErrInvalidStyle) {
			t.Fatalf("LayoutContext(%+v) = %v, %v, want nil, ErrInvalidStyle", style, box, err)
		}
	}

	var nilEngine *Engine
	if box, err := nilEngine.Layout(expr, Style{Size: 16}); box != nil || !errors.Is(err, ErrInvalidEngine) {
		t.Fatalf("nil Engine.Layout() = %v, %v", box, err)
	}
	//nolint:staticcheck // Intentionally verify nil-context handling at the public boundary.
	if box, err := prepared.LayoutContext(nil, expr, Style{Size: 16}); box != nil || !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("nil context = %v, %v", box, err)
	}
	if box, err := prepared.Layout(nil, Style{Size: 16}); box != nil || !errors.Is(err, ErrInvalidExpr) {
		t.Fatalf("nil expression = %v, %v", box, err)
	}
	if box, err := Layout(nil, fonts, Style{Size: 16}); box != nil || !errors.Is(err, ErrInvalidExpr) {
		t.Fatalf("nil convenience expression = %v, %v", box, err)
	}
}

func TestLayoutAlignmentContract(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := Parse("x", TeX, Inline, ParseOptions{})
	start, err := Layout(expr, fonts, Style{Size: 16, MaxWidth: 1, Align: display.AlignStart})
	if err != nil {
		t.Fatal(err)
	}
	end, err := Layout(expr, fonts, Style{Size: 16, MaxWidth: 1, Align: display.AlignEnd})
	if err != nil {
		t.Fatal(err)
	}
	if startCanonical, endCanonical := mustCanonical(t, start), mustCanonical(t, end); !bytes.Equal(startCanonical, endCanonical) {
		t.Fatal("inline layout did not ignore Style.Align")
	}

	displayExpr := Parse("x", TeX, Display, ParseOptions{})
	unconstrained, err := Layout(displayExpr, fonts, Style{Size: 16, Align: display.AlignEnd})
	if err != nil {
		t.Fatal(err)
	}
	if len(unconstrained.Lines) != 0 {
		t.Fatalf("zero-MaxWidth Lines = %d, want none", len(unconstrained.Lines))
	}
	center, err := Layout(displayExpr, fonts, Style{Size: 16, MaxWidth: 100, Align: display.AlignCenter})
	if err != nil {
		t.Fatal(err)
	}
	endDisplay, err := Layout(displayExpr, fonts, Style{Size: 16, MaxWidth: 100, Align: display.AlignEnd})
	if err != nil {
		t.Fatal(err)
	}
	if len(center.Lines) != 1 || len(endDisplay.Lines) != 1 {
		t.Fatalf("constrained Lines = %d/%d, want one each", len(center.Lines), len(endDisplay.Lines))
	}
	if center.Width != 100 || endDisplay.Width != 100 {
		t.Fatalf("constrained widths = %v/%v, want frame 100", center.Width, endDisplay.Width)
	}
	if center.Lines[0].OffsetX <= 0 || endDisplay.Lines[0].OffsetX <= center.Lines[0].OffsetX {
		t.Fatalf("center/end offsets = %v/%v, want positive and increasing",
			center.Lines[0].OffsetX, endDisplay.Lines[0].OffsetX)
	}
	if centerCanonical, endCanonical := mustCanonical(t, center), mustCanonical(t, endDisplay); bytes.Equal(centerCanonical, endCanonical) {
		t.Fatal("display alignment did not change canonical output")
	}
}

func TestLayoutTeXQuadReportsLogicalOverflow(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := ParseTeX(`\quad`, Display, ParseOptions{})
	unwrapped, err := Layout(expr, fonts, Style{Size: 16})
	if err != nil {
		t.Fatal(err)
	}
	box, err := Layout(expr, fonts, Style{Size: 16, MaxWidth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(box.Lines) != 1 {
		t.Fatalf("Lines = %d, want one constrained line", len(box.Lines))
	}
	want := unwrapped.Width - 1
	assertHostClose(t, box.Lines[0].Box.Overflow, want)
	assertHostClose(t, box.Overflow, want)
}

func assertHostClose(t *testing.T, got, want float32) {
	t.Helper()
	const tolerance = float32(1e-4)
	if diff := got - want; diff > tolerance || diff < -tolerance {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func mustCanonical(t testing.TB, box *Box) []byte {
	t.Helper()
	data, err := box.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	return data
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
