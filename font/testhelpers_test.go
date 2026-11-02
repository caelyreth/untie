package font

import (
	"encoding/binary"
	"os"
	"sync"
	"testing"
)

const (
	testFontPath    = "testdata/NewCMMath/NewCMMath-Book.otf"
	testFixturePath = "testdata/NewCMMath/Typlet/font.jsonl"
)

var (
	testFontOnce sync.Once
	testFontSet  *FontSet
	testFontErr  error
)

func loadSharedTestFont(t testing.TB) *FontSet {
	t.Helper()
	testFontOnce.Do(func() {
		data, err := os.ReadFile(testFontPath)
		if err != nil {
			testFontErr = err
			return
		}
		testFontSet, testFontErr = Load(Fonts{
			Math: FontData{Bytes: data, ID: "newcm-math-book", Index: 0},
		})
	})
	if testFontErr != nil {
		t.Fatalf("load NewCM math font: %v", testFontErr)
	}
	return testFontSet
}

func readTestFont(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile(testFontPath)
	if err != nil {
		t.Fatalf("read NewCM math font: %v", err)
	}
	return data
}

// makeTTC wraps the same SFNT face in a collection with n independent directory
// records. It is deliberately built from the tracked test font so the normal
// suite does not depend on an external collection fixture.
func makeTTC(t *testing.T, src []byte, n int) []byte {
	t.Helper()
	if n <= 0 {
		t.Fatal("makeTTC requires at least one face")
	}
	if len(src) < sfntHeaderLen {
		t.Fatal("source font is truncated")
	}
	numTables := int(binary.BigEndian.Uint16(src[4:6]))
	if numTables <= 0 {
		t.Fatal("source font has no tables")
	}
	headerLen := 12 + 4*n
	dirLen := sfntHeaderLen + tableRecordLen*numTables
	dataStart := headerLen + n*dirLen
	var dataLen int
	for i := 0; i < numTables; i++ {
		off := sfntHeaderLen + i*tableRecordLen
		dataLen += int(align4(uint64(binary.BigEndian.Uint32(src[off+12 : off+16]))))
	}
	out := make([]byte, dataStart+dataLen)
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], 0x00010000)
	binary.BigEndian.PutUint32(out[8:12], uint32(n))

	tableDataOff := dataStart
	offsets := make([]uint32, numTables)
	for i := 0; i < numTables; i++ {
		off := sfntHeaderLen + i*tableRecordLen
		length := int(binary.BigEndian.Uint32(src[off+12 : off+16]))
		srcOff := int(binary.BigEndian.Uint32(src[off+8 : off+12]))
		offsets[i] = uint32(tableDataOff)
		copy(out[tableDataOff:tableDataOff+length], src[srcOff:srcOff+length])
		tableDataOff += int(align4(uint64(length)))
	}

	for face := 0; face < n; face++ {
		faceOff := headerLen + face*dirLen
		binary.BigEndian.PutUint32(out[12+4*face:16+4*face], uint32(faceOff))
		copy(out[faceOff:faceOff+4], src[0:4])
		copy(out[faceOff+4:faceOff+12], src[4:12])
		for i := 0; i < numTables; i++ {
			srcRec := sfntHeaderLen + i*tableRecordLen
			dstRec := faceOff + sfntHeaderLen + i*tableRecordLen
			copy(out[dstRec:dstRec+8], src[srcRec:srcRec+8])
			binary.BigEndian.PutUint32(out[dstRec+8:dstRec+12], offsets[i])
			copy(out[dstRec+12:dstRec+16], src[srcRec+12:srcRec+16])
		}
	}
	return out
}
