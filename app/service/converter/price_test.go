package converter

import (
	"strings"
	"testing"

	math2 "cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
)

// testAsset builds a chain-registry asset whose display unit carries the given
// exponent, mirroring the shape the registry returns (base unit at 0, display
// unit at the asset's decimals).
func testAsset(denom string, exponent int) *chain_registry.ChainRegistryAsset {
	return &chain_registry.ChainRegistryAsset{
		Base:    denom,
		Display: "display" + denom,
		Symbol:  strings.ToUpper(denom),
		DenomUnits: []chain_registry.ChainRegistryAssetDenom{
			{Denom: denom, Exponent: 0},
			{Denom: "display" + denom, Exponent: exponent},
		},
	}
}

// assetWithoutDisplayUnit has a Display that matches none of its denom units,
// so GetDisplayDenomUnit returns nil.
func assetWithoutDisplayUnit(denom string) *chain_registry.ChainRegistryAsset {
	return &chain_registry.ChainRegistryAsset{
		Base:       denom,
		Display:    "missing",
		DenomUnits: []chain_registry.ChainRegistryAssetDenom{{Denom: denom, Exponent: 0}},
	}
}

func TestUPriceToPrice(t *testing.T) {
	cases := []struct {
		name      string
		baseExp   int
		quoteExp  int
		price     string
		want      string
		wantFloat float64
	}{
		{name: "equal exponents pass the price through and trim", baseExp: 6, quoteExp: 6, price: "0.00012340", want: "0.0001234", wantFloat: 0.0001234},
		{name: "equal exponents keep an integer price as is", baseExp: 6, quoteExp: 6, price: "12", want: "12", wantFloat: 12},
		{name: "base exponent above quote scales up", baseExp: 6, quoteExp: 0, price: "0.00012340", want: "123.4", wantFloat: 123.4},
		{name: "base exponent above quote, gap of 6", baseExp: 12, quoteExp: 6, price: "0.00012340", want: "123.4", wantFloat: 123.4},
		{name: "base exponent below quote by 1 scales down", baseExp: 6, quoteExp: 7, price: "0.00012340", want: "0.00001234", wantFloat: 1.234e-05},
		{name: "base exponent below quote by 2 scales down", baseExp: 6, quoteExp: 8, price: "0.000001234", want: "0.00000001234", wantFloat: 1.234e-08},
		{name: "large gap upwards", baseExp: 18, quoteExp: 0, price: "0.00012340", want: "123400000000000", wantFloat: 1.234e+14},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFloat, err := UPriceToPrice(testAsset("aaa", tc.baseExp), testAsset("bbb", tc.quoteExp), tc.price)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected price %q, got %q", tc.want, got)
			}

			if gotFloat != tc.wantFloat {
				t.Fatalf("expected float price %v, got %v", tc.wantFloat, gotFloat)
			}
		})
	}
}

// BUG: the multiplier is built as fmt.Sprintf("%.2f", math.Pow10(base-quote)),
// so any exponent gap of -3 or more rounds the multiplier to "0.00" and the
// price collapses to zero. Filed as BZE-133; the assertions below pin the
// current behaviour so the fix is visible as a test change.
func TestUPriceToPriceZeroesPricesOnLargeDownwardGaps(t *testing.T) {
	for _, tc := range []struct {
		name     string
		baseExp  int
		quoteExp int
	}{
		{name: "gap of 3", baseExp: 6, quoteExp: 9},
		{name: "gap of 6", baseExp: 6, quoteExp: 12},
		{name: "gap of 18", baseExp: 0, quoteExp: 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFloat, err := UPriceToPrice(testAsset("aaa", tc.baseExp), testAsset("bbb", tc.quoteExp), "0.00012340")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != "0" || gotFloat != 0 {
				t.Fatalf("expected the price to collapse to 0 (BZE-133), got %q / %v", got, gotFloat)
			}
		})
	}
}

// BUG: math.Pow10 goes through float64, so exponent gaps beyond 22 lose
// precision — 0.0001234 * 1e24 should be 123400000000000000000. Part of BZE-133.
func TestUPriceToPriceIsLossyOnVeryLargeUpwardGaps(t *testing.T) {
	got, _, err := UPriceToPrice(testAsset("aaa", 24), testAsset("bbb", 0), "0.00012340")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "123399999999999997929.6915456" {
		t.Fatalf("expected the float64 rounding error to show up (BZE-133), got %q", got)
	}
}

func TestUPriceToPriceErrors(t *testing.T) {
	t.Run("base without a display unit", func(t *testing.T) {
		if _, _, err := UPriceToPrice(assetWithoutDisplayUnit("aaa"), testAsset("bbb", 6), "1.5"); err == nil {
			t.Fatalf("expected an error for a base asset without a display unit")
		}
	})

	t.Run("quote without a display unit", func(t *testing.T) {
		if _, _, err := UPriceToPrice(testAsset("aaa", 6), assetWithoutDisplayUnit("bbb"), "1.5"); err == nil {
			t.Fatalf("expected an error for a quote asset without a display unit")
		}
	})

	t.Run("unparsable price", func(t *testing.T) {
		if _, _, err := UPriceToPrice(testAsset("aaa", 6), testAsset("bbb", 6), "abc"); err == nil {
			t.Fatalf("expected an error for an unparsable price")
		}
	})
}

func TestUAmountToAmount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exponent int
		amount   string
		want     string
	}{
		{name: "exponent 0 leaves the amount untouched", exponent: 0, amount: "1500000", want: "1500000"},
		{name: "exponent 6", exponent: 6, amount: "1500000", want: "1.5"},
		{name: "exponent 12 (LP tokens)", exponent: 12, amount: "1500000000000", want: "1.5"},
		{name: "exponent 18", exponent: 18, amount: "100", want: "0.0000000000000001"},
		{name: "zero", exponent: 6, amount: "0", want: "0"},
		{name: "negative", exponent: 6, amount: "-1500000", want: "-1.5"},
		// used to overflow: the code converted through int64 before this ticket
		{name: "above the int64 range", exponent: 6, amount: "9223372036854775808", want: "9223372036854.775808"},
		{name: "far above the int64 range", exponent: 18, amount: "170141183460469231731687303715884105727", want: "170141183460469231731.687303715884105727"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := UAmountToAmount(testAsset("aaa", tc.exponent), tc.amount)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestUAmountToAmountErrors(t *testing.T) {
	t.Run("asset without a display unit", func(t *testing.T) {
		if _, err := UAmountToAmount(assetWithoutDisplayUnit("aaa"), "100"); err == nil {
			t.Fatalf("expected an error for an asset without a display unit")
		}
	})

	// used to panic with a nil pointer dereference: NewIntFromString's ok flag
	// was discarded
	t.Run("unparsable amount", func(t *testing.T) {
		if _, err := UAmountToAmount(testAsset("aaa", 6), "notanumber"); err == nil {
			t.Fatalf("expected an error for an unparsable amount")
		}
	})

	t.Run("empty amount", func(t *testing.T) {
		if _, err := UAmountToAmount(testAsset("aaa", 6), ""); err == nil {
			t.Fatalf("expected an error for an empty amount")
		}
	})
}

// LegacyDec caps precision at 18 decimals, so an asset declaring a larger
// exponent panics. No chain-registry asset does, and guarding it would mean
// inventing a fallback, so the limit is pinned here instead.
func TestUAmountToAmountPanicsAboveMaxPrecision(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic for an exponent above 18")
		}
	}()

	_, _ = UAmountToAmount(testAsset("aaa", 19), "100")
}

func TestTrimAmountTrailingZeros(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{in: "1.100", want: "1.1"},
		{in: "1.000", want: "1"},
		{in: "100", want: "100"},
		{in: "1000", want: "1000"},
		{in: "0.000001", want: "0.000001"},
		{in: "0.000000", want: "0"},
		{in: "-1.10", want: "-1.1"},
		{in: "", want: ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			if got := TrimAmountTrailingZeros(tc.in); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDecToFloat32Rounded(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want float32
	}{
		{name: "rounds to two decimals", in: "123.456", want: 123.46},
		{name: "keeps zero", in: "0", want: 0},
		{name: "negative rounds away from zero", in: "-1.006", want: -1.01},
		// 1.005 is not representable in binary: the nearest float64 sits just
		// below it, so it rounds down while 2.675 (nearest float64 just above)
		// rounds up
		{name: "positive .005 boundary rounds down", in: "1.005", want: 1},
		{name: "negative .005 boundary rounds towards zero", in: "-1.005", want: -1},
		{name: "other .005 boundary rounds up", in: "2.675", want: 2.68},
		{name: "just below the boundary", in: "1.004999", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecToFloat32Rounded(math2.LegacyMustNewDecFromStr(tc.in)); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
