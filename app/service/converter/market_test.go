package converter

import (
	"testing"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
)

// a real pool: the base denom itself contains slashes, which is why market ids
// (slash separated) and pool ids (underscore separated) cannot be interchanged
const (
	factoryDenom = "factory/bze13gzq40che93tgfm9kzmkpjamah5nj0j73pyhqk/uvdl"
	nativeDenom  = "ubze"
)

func TestGetMarketId(t *testing.T) {
	for _, tc := range []struct {
		name        string
		base, quote string
		want        string
	}{
		{name: "plain denoms", base: "utbz", quote: "ubze", want: "utbz/ubze"},
		{name: "factory base keeps its own slashes", base: factoryDenom, quote: nativeDenom, want: factoryDenom + "/" + nativeDenom},
		{name: "order is preserved, not sorted", base: "ubze", quote: "utbz", want: "ubze/utbz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetMarketId(tc.base, tc.quote); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestCreatePoolId(t *testing.T) {
	for _, tc := range []struct {
		name        string
		base, quote string
		want        string
	}{
		{name: "already ordered", base: "aaa", quote: "bbb", want: "aaa_bbb"},
		{name: "swapped into lexicographic order", base: "bbb", quote: "aaa", want: "aaa_bbb"},
		{name: "factory denom sorts before the native one", base: factoryDenom, quote: nativeDenom, want: factoryDenom + "_" + nativeDenom},
		{name: "same input in the other order yields the same id", base: nativeDenom, quote: factoryDenom, want: factoryDenom + "_" + nativeDenom},
		{name: "equal denoms", base: "aaa", quote: "aaa", want: "aaa_aaa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CreatePoolId(tc.base, tc.quote); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// a pool id built from either direction must round-trip back to its denoms
func TestCreatePoolIdRoundTripsThroughPoolIdToDenoms(t *testing.T) {
	id := CreatePoolId(nativeDenom, factoryDenom)

	base, quote, err := PoolIdToDenoms(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if base != factoryDenom || quote != nativeDenom {
		t.Fatalf("expected %q/%q, got %q/%q", factoryDenom, nativeDenom, base, quote)
	}
}

func TestIsLpDenom(t *testing.T) {
	for _, tc := range []struct {
		denom string
		want  bool
	}{
		{denom: "ulp_aaa_bbb", want: true},
		{denom: "ulp_" + factoryDenom + "_" + nativeDenom, want: true},
		{denom: "ubze", want: false},
		{denom: "ulp", want: false},
		{denom: "", want: false},
		{denom: "xulp_aaa_bbb", want: false},
	} {
		t.Run(tc.denom, func(t *testing.T) {
			if got := IsLpDenom(tc.denom); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestPoolIdFromPoolDenom(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{in: "ulp_aaa_bbb", want: "aaa_bbb"},
		{in: "ulp_" + factoryDenom + "_" + nativeDenom, want: factoryDenom + "_" + nativeDenom},
		// only the leading prefix is removed, non-LP denoms are returned as is
		{in: "ubze", want: "ubze"},
		{in: "", want: ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			if got := PoolIdFromPoolDenom(tc.in); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestGetLpAssetDecimals(t *testing.T) {
	if got := GetLpAssetDecimals(); got != 12 {
		t.Fatalf("expected LP tokens to carry 12 decimals, got %d", got)
	}
}

func TestGetQuoteAmount(t *testing.T) {
	for _, tc := range []struct {
		name       string
		baseAmount string
		price      string
		quote      *chain_registry.ChainRegistryAsset
		want       string
	}{
		{name: "truncates to the quote exponent", baseAmount: "1.5", price: "0.123456789", quote: testAsset("bbb", 6), want: "0.185185"},
		{name: "a smaller exponent truncates harder", baseAmount: "1.5", price: "0.123456789", quote: testAsset("bbb", 2), want: "0.18"},
		{name: "exponent 0 truncates to whole units", baseAmount: "1.5", price: "0.9", quote: testAsset("bbb", 0), want: "1"},
		{name: "no display unit falls back to 6 decimals", baseAmount: "1.5", price: "0.123456789", quote: assetWithoutDisplayUnit("bbb"), want: "0.185185"},
		{name: "exact result keeps no trailing zeros", baseAmount: "2", price: "0.5", quote: testAsset("bbb", 6), want: "1"},
		{name: "zero amount", baseAmount: "0", price: "1.23", quote: testAsset("bbb", 6), want: "0"},
		{name: "truncates rather than rounds", baseAmount: "1", price: "0.9999999", quote: testAsset("bbb", 6), want: "0.999999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetQuoteAmount(tc.baseAmount, tc.price, tc.quote); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// GetQuoteAmount cannot report errors — it parses with the Must* helpers, so a
// malformed amount panics. Callers only ever feed it converter output.
func TestGetQuoteAmountPanicsOnUnparsableInput(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic for an unparsable base amount")
		}
	}()

	GetQuoteAmount("not-a-number", "1", testAsset("bbb", 6))
}
