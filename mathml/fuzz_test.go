package mathml_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/caelyreth/untie/mathml"
)

func FuzzUnmarshalAcceptedOutputRoundTrips(f *testing.F) {
	for _, seed := range []string{
		`<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`,
		`<math xmlns="http://www.w3.org/1998/Math/MathML"><mrow><mfrac><mn>1</mn><mn>2</mn></mfrac></mrow></math>`,
		`<math xmlns="http://www.w3.org/1998/Math/MathML"><mfenced><mi>a</mi><mi>b</mi></mfenced></math>`,
		`<math xmlns="http://www.w3.org/1998/Math/MathML"><mtable><mtr><mtd><mi>x</mi></mtd></mtr></mtable></math>`,
		`<math><ms>unsupported</ms></math>`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		first, err := mathml.Unmarshal(context.Background(), data, mathml.Options{})
		if err != nil {
			if first.Formula != nil {
				t.Fatalf("error returned with a formula: %v", err)
			}
			if len(first.Diagnostics) == 0 {
				t.Fatalf("error %v was not accompanied by a diagnostic", err)
			}
			return
		}
		if first.Formula == nil {
			t.Fatal("successful import returned nil formula")
		}
		output, err := mathml.Marshal(context.Background(), first.Formula, first.Mode, mathml.Options{})
		if err != nil {
			t.Fatalf("accepted formula could not be marshaled: %v", err)
		}
		second, err := mathml.Unmarshal(context.Background(), output, mathml.Options{})
		if err != nil {
			t.Fatalf("own output could not be imported: %v\n%s", err, output)
		}
		firstCanonical, err := normalizedCanonicalValue(first.Formula)
		if err != nil {
			t.Fatal(err)
		}
		secondCanonical, err := normalizedCanonicalValue(second.Formula)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(firstCanonical, secondCanonical) {
			t.Fatalf("round-trip changed semantics\nfirst: %s\nsecond: %s\nMathML: %s",
				firstCanonical, secondCanonical, output)
		}
	})
}
