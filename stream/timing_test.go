package stream

import "testing"

func TestTimingCurvesAreMonotonicAndNonLinear(t *testing.T) {
	t.Parallel()

	for _, timing := range []Timing{TimingEaseOutCubic, TimingEaseInOutCubic} {
		if got := timing.apply(0); got != 0 {
			t.Fatalf("timing %d at zero = %v", timing, got)
		}
		if got := timing.apply(1); got != 1 {
			t.Fatalf("timing %d at one = %v", timing, got)
		}
		previous := float32(0)
		for i := 1; i <= 100; i++ {
			value := timing.apply(float32(i) / 100)
			if value < previous {
				t.Fatalf("timing %d is not monotonic at %d: %v < %v",
					timing, i, value, previous)
			}
			previous = value
		}
		if timing.apply(0.25) == 0.25 {
			t.Fatalf("timing %d is linear", timing)
		}
	}
	if TimingEaseOutCubic.apply(0.25) <= TimingEaseInOutCubic.apply(0.25) {
		t.Fatal("ease-out does not lead ease-in-out early in the transition")
	}
}
