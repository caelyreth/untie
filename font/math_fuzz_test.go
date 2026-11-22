package font

import "testing"

func FuzzParseMathKernInfo(f *testing.F) {
	f.Add(mathKernInfoFixture())
	f.Add([]byte{0, 4, 0, 0, 0, 1, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseMathKernInfo(data)
	})
}
