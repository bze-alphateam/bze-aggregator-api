package calculator

import (
	"testing"

	"cosmossdk.io/math"
)

func TestCalculatePriceChange(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opening string
		last    string
		want    string
	}{
		{name: "price went up by ten percent", opening: "100", last: "110", want: "10.000000000000000000"},
		{name: "price went down by ten percent", opening: "100", last: "90", want: "-10.000000000000000000"},
		{name: "price doubled", opening: "0.5", last: "1", want: "100.000000000000000000"},
		{name: "no change", opening: "100", last: "100", want: "0.000000000000000000"},
		{name: "price collapsed to zero", opening: "100", last: "0", want: "-100.000000000000000000"},
		{name: "fractional prices", opening: "0.00012340", last: "0.00013000", want: "5.348460291734197700"},
		// division by zero is guarded: a market with no opening price reports
		// no change rather than blowing up
		{name: "zero opening price", opening: "0", last: "110", want: "0.000000000000000000"},
		{name: "negative opening price", opening: "-100", last: "110", want: "0.000000000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculatePriceChange(math.LegacyMustNewDecFromStr(tc.opening), math.LegacyMustNewDecFromStr(tc.last))
			if got.String() != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
