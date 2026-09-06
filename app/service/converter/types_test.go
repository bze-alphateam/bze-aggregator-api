package converter

import (
	"errors"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// the base and the quote deliberately carry different exponents, so a
// converter that mixed the two up would produce different numbers and fail
const (
	testBaseDenom  = "aaa"
	testBaseExp    = 6
	testQuoteDenom = "bbb"
	testQuoteExp   = 8
)

type fakeAssetProvider struct {
	assets map[string]*chain_registry.ChainRegistryAsset
	err    error
	calls  []string
}

func (p *fakeAssetProvider) GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error) {
	p.calls = append(p.calls, denom)
	if p.err != nil {
		return nil, p.err
	}

	// an unknown denom is reported as a nil asset without an error
	return p.assets[denom], nil
}

func testProvider() *fakeAssetProvider {
	return &fakeAssetProvider{
		assets: map[string]*chain_registry.ChainRegistryAsset{
			testBaseDenom:  testAsset(testBaseDenom, testBaseExp),
			testQuoteDenom: testAsset(testQuoteDenom, testQuoteExp),
		},
	}
}

func testConverter(t *testing.T) *TypesConverter {
	t.Helper()

	tc, err := NewTypesConverter(testProvider(), testBaseDenom, testQuoteDenom)
	if err != nil {
		t.Fatalf("unexpected error building the converter: %v", err)
	}

	return tc
}

func TestNewTypesConverter(t *testing.T) {
	provider := testProvider()

	tc, err := NewTypesConverter(provider, testBaseDenom, testQuoteDenom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tc.base.Base != testBaseDenom || tc.quote.Base != testQuoteDenom {
		t.Fatalf("expected the assets to be looked up in order, got %q/%q", tc.base.Base, tc.quote.Base)
	}

	if len(provider.calls) != 2 || provider.calls[0] != testBaseDenom || provider.calls[1] != testQuoteDenom {
		t.Fatalf("expected one lookup per denom, got %v", provider.calls)
	}
}

func TestNewTypesConverterErrors(t *testing.T) {
	providerErr := errors.New("registry unavailable")

	t.Run("provider failure", func(t *testing.T) {
		provider := testProvider()
		provider.err = providerErr

		_, err := NewTypesConverter(provider, testBaseDenom, testQuoteDenom)
		if !errors.Is(err, providerErr) {
			t.Fatalf("expected the provider error to be returned, got %v", err)
		}
	})

	t.Run("unknown base", func(t *testing.T) {
		if _, err := NewTypesConverter(testProvider(), "unknown", testQuoteDenom); err == nil {
			t.Fatalf("expected an error for an unknown base denom")
		}
	})

	t.Run("unknown quote", func(t *testing.T) {
		if _, err := NewTypesConverter(testProvider(), testBaseDenom, "unknown"); err == nil {
			t.Fatalf("expected an error for an unknown quote denom")
		}
	})
}

func swapData(inDenom string, in int64, outDenom string, out int64) dto.SwapEventData {
	return dto.SwapEventData{
		EventID:    77,
		PoolID:     testBaseDenom + "_" + testQuoteDenom,
		Creator:    "bze1creator",
		Input:      sdk.NewCoin(inDenom, math.NewInt(in)),
		Output:     sdk.NewCoin(outDenom, math.NewInt(out)),
		ExecutedAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func TestSwapDataToHistoryEntity(t *testing.T) {
	for _, tc := range []struct {
		name          string
		source        dto.SwapEventData
		wantOrderType string
	}{
		// paying in the base denom sells it for the quote
		{name: "input is the base denom", source: swapData(testBaseDenom, 2000000, testQuoteDenom, 4000000), wantOrderType: "sell"},
		{name: "input is the quote denom", source: swapData(testQuoteDenom, 4000000, testBaseDenom, 2000000), wantOrderType: "buy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := testConverter(t).SwapDataToHistoryEntity(tc.source)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.OrderType != tc.wantOrderType {
				t.Fatalf("expected order type %q, got %q", tc.wantOrderType, got.OrderType)
			}

			// 2000000 base units at 6 decimals
			if got.Amount != "2" {
				t.Fatalf("expected the base amount to use the base decimals, got %q", got.Amount)
			}

			// 4000000 quote units at 8 decimals
			if got.QuoteAmount != "0.04" {
				t.Fatalf("expected the quote amount to use the quote decimals, got %q", got.QuoteAmount)
			}

			// raw ratio 2, rescaled by 10^(6-8)
			if got.Price != "0.02" {
				t.Fatalf("expected price %q, got %q", "0.02", got.Price)
			}

			if got.MarketID != tc.source.PoolID || got.Taker != "bze1creator" {
				t.Fatalf("expected the swap metadata to be carried over, got %+v", got)
			}

			if !got.ExecutedAt.Equal(tc.source.ExecutedAt) {
				t.Fatalf("unexpected execution time: %s", got.ExecutedAt)
			}
		})
	}
}

func TestSwapDataToHistoryEntityRejectsZeroBase(t *testing.T) {
	_, err := testConverter(t).SwapDataToHistoryEntity(swapData(testBaseDenom, 0, testQuoteDenom, 4000000))
	if err == nil {
		t.Fatalf("expected an error when the base amount is zero")
	}
}

func TestHistoryOrderToHistoryEntity(t *testing.T) {
	got, err := testConverter(t).HistoryOrderToHistoryEntity(&tradebinTypes.HistoryOrder{
		MarketId:   testBaseDenom + "/" + testQuoteDenom,
		OrderType:  "buy",
		Amount:     "2000000",
		Price:      "0.5",
		ExecutedAt: 1725000000,
		Maker:      "bze1maker",
		Taker:      "bze1taker",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 0.5 rescaled by 10^(6-8)
	if got.Price != "0.005" {
		t.Fatalf("expected price %q, got %q", "0.005", got.Price)
	}

	if got.Amount != "2" {
		t.Fatalf("expected the base amount to use the base decimals, got %q", got.Amount)
	}

	// 2 * 0.005, truncated to the quote's 8 decimals
	if got.QuoteAmount != "0.01" {
		t.Fatalf("expected quote amount %q, got %q", "0.01", got.QuoteAmount)
	}

	if got.MarketID != testBaseDenom+"/"+testQuoteDenom || got.OrderType != "buy" {
		t.Fatalf("expected the order metadata to be carried over, got %+v", got)
	}

	if got.Maker != "bze1maker" || got.Taker != "bze1taker" {
		t.Fatalf("unexpected participants: %q/%q", got.Maker, got.Taker)
	}
}

func TestHistoryOrderToHistoryEntityErrors(t *testing.T) {
	t.Run("unparsable price", func(t *testing.T) {
		_, err := testConverter(t).HistoryOrderToHistoryEntity(&tradebinTypes.HistoryOrder{Amount: "2000000", Price: "abc"})
		if err == nil {
			t.Fatalf("expected an error for an unparsable price")
		}
	})

	t.Run("unparsable amount", func(t *testing.T) {
		_, err := testConverter(t).HistoryOrderToHistoryEntity(&tradebinTypes.HistoryOrder{Amount: "abc", Price: "0.5"})
		if err == nil {
			t.Fatalf("expected an error for an unparsable amount")
		}
	})
}

func TestAggregatedOrderToOrderEntity(t *testing.T) {
	got, err := testConverter(t).AggregatedOrderToOrderEntity(&tradebinTypes.AggregatedOrder{
		MarketId:  testBaseDenom + "/" + testQuoteDenom,
		OrderType: "sell",
		Amount:    "2000000",
		Price:     "0.5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Price != "0.005" {
		t.Fatalf("expected price %q, got %q", "0.005", got.Price)
	}

	// the float mirror of the price is what the order book sorts on
	if got.PriceDec != 0.005 {
		t.Fatalf("expected price_dec %v, got %v", 0.005, got.PriceDec)
	}

	if got.Amount != "2" {
		t.Fatalf("expected the base amount to use the base decimals, got %q", got.Amount)
	}

	if got.QuoteAmount != "0.01" {
		t.Fatalf("expected quote amount %q, got %q", "0.01", got.QuoteAmount)
	}

	if got.MarketID != testBaseDenom+"/"+testQuoteDenom || got.OrderType != "sell" {
		t.Fatalf("expected the order metadata to be carried over, got %+v", got)
	}
}

func TestAggregatedOrderToOrderEntityErrors(t *testing.T) {
	t.Run("unparsable price", func(t *testing.T) {
		_, err := testConverter(t).AggregatedOrderToOrderEntity(&tradebinTypes.AggregatedOrder{Amount: "2000000", Price: "abc"})
		if err == nil {
			t.Fatalf("expected an error for an unparsable price")
		}
	})

	t.Run("unparsable amount", func(t *testing.T) {
		_, err := testConverter(t).AggregatedOrderToOrderEntity(&tradebinTypes.AggregatedOrder{Amount: "abc", Price: "0.5"})
		if err == nil {
			t.Fatalf("expected an error for an unparsable amount")
		}
	})
}

// the base and the quote must not be interchangeable: the same numbers with the
// assets swapped have to produce a different result
func TestTypesConverterDistinguishesBaseFromQuote(t *testing.T) {
	swapped, err := NewTypesConverter(testProvider(), testQuoteDenom, testBaseDenom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := &tradebinTypes.AggregatedOrder{MarketId: "x", OrderType: "buy", Amount: "2000000", Price: "0.5"}

	straight, err := testConverter(t).AggregatedOrderToOrderEntity(order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reversed, err := swapped.AggregatedOrderToOrderEntity(order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if straight.Price == reversed.Price || straight.Amount == reversed.Amount {
		t.Fatalf("expected swapping base and quote to change the result, got %+v and %+v", straight, reversed)
	}
}

// the quote lookup happens after the base one, so its failure needs a provider
// that only breaks on the second call
type quoteFailingProvider struct {
	inner *fakeAssetProvider
	err   error
}

func (p *quoteFailingProvider) GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error) {
	if denom == testQuoteDenom {
		return nil, p.err
	}

	return p.inner.GetAssetDetails(denom)
}

func TestNewTypesConverterReturnsQuoteLookupErrors(t *testing.T) {
	providerErr := errors.New("registry unavailable")

	_, err := NewTypesConverter(&quoteFailingProvider{inner: testProvider(), err: providerErr}, testBaseDenom, testQuoteDenom)
	if !errors.Is(err, providerErr) {
		t.Fatalf("expected the provider error to be returned, got %v", err)
	}
}

// NewTypesConverter does not require the assets to carry a display unit, so the
// missing decimals only surface once a conversion is attempted
func TestSwapDataToHistoryEntityFailsWithoutDisplayUnits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		assets map[string]*chain_registry.ChainRegistryAsset
	}{
		{
			name: "base without a display unit",
			assets: map[string]*chain_registry.ChainRegistryAsset{
				testBaseDenom:  assetWithoutDisplayUnit(testBaseDenom),
				testQuoteDenom: testAsset(testQuoteDenom, testQuoteExp),
			},
		},
		{
			name: "quote without a display unit",
			assets: map[string]*chain_registry.ChainRegistryAsset{
				testBaseDenom:  testAsset(testBaseDenom, testBaseExp),
				testQuoteDenom: assetWithoutDisplayUnit(testQuoteDenom),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			converter, err := NewTypesConverter(&fakeAssetProvider{assets: tc.assets}, testBaseDenom, testQuoteDenom)
			if err != nil {
				t.Fatalf("unexpected error building the converter: %v", err)
			}

			if _, err := converter.SwapDataToHistoryEntity(swapData(testBaseDenom, 2000000, testQuoteDenom, 4000000)); err == nil {
				t.Fatalf("expected an error when the asset decimals are unknown")
			}
		})
	}
}
