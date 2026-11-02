package geometry

import "math/bits"

func bitsMul(a, b uint64) (hi, lo uint64) {
	return bits.Mul64(a, b)
}

func bitsDiv(hi, lo, divisor uint64) (q, r uint64) {
	return bits.Div64(hi, lo, divisor)
}
