package converter

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestNewMarketEntity(t *testing.T) {
	before := time.Now()

	got := NewMarketEntity(&tradebinTypes.Market{
		Base:    factoryDenom,
		Quote:   nativeDenom,
		Creator: "bze1creator",
	})

	if got.MarketID != factoryDenom+"/"+nativeDenom {
		t.Fatalf("unexpected market id: %q", got.MarketID)
	}

	if got.Base != factoryDenom || got.Quote != nativeDenom {
		t.Fatalf("unexpected denoms: %q/%q", got.Base, got.Quote)
	}

	if got.CreatedBy != "bze1creator" {
		t.Fatalf("unexpected creator: %q", got.CreatedBy)
	}

	// CreatedAt is stamped locally, not taken from the chain
	if got.CreatedAt.Before(before) || got.CreatedAt.After(time.Now()) {
		t.Fatalf("expected CreatedAt to be stamped now, got %s", got.CreatedAt)
	}
}

// the proto getters are nil safe, so a missing market degrades to empty fields
// rather than panicking
func TestNewMarketEntityFromNil(t *testing.T) {
	got := NewMarketEntity(nil)

	if got.MarketID != "/" || got.Base != "" || got.Quote != "" || got.CreatedBy != "" {
		t.Fatalf("unexpected entity for a nil market: %+v", got)
	}
}

func TestNewMarketOrderEntity(t *testing.T) {
	got, err := NewMarketOrderEntity(&tradebinTypes.AggregatedOrder{
		MarketId:  "aaa/bbb",
		OrderType: "buy",
		Amount:    "2000000",
		Price:     "0.5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.MarketID != "aaa/bbb" || got.OrderType != "buy" || got.Price != "0.5" {
		t.Fatalf("unexpected entity: %+v", got)
	}

	// the amount is not carried over here — TypesConverter fills it in after
	// applying the asset decimals
	if got.Amount != "" {
		t.Fatalf("expected the amount to be left empty, got %q", got.Amount)
	}
}

func TestNewMarketHistoryEntity(t *testing.T) {
	got, err := NewMarketHistoryEntity(&tradebinTypes.HistoryOrder{
		MarketId:   "aaa/bbb",
		OrderType:  "sell",
		Amount:     "2000000",
		Price:      "0.5",
		ExecutedAt: 1725000000,
		Maker:      "bze1maker",
		Taker:      "bze1taker",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.MarketID != "aaa/bbb" || got.OrderType != "sell" || got.Price != "0.5" {
		t.Fatalf("unexpected entity: %+v", got)
	}

	if got.Maker != "bze1maker" || got.Taker != "bze1taker" {
		t.Fatalf("unexpected participants: %q/%q", got.Maker, got.Taker)
	}

	// the chain reports seconds, not milliseconds
	if !got.ExecutedAt.Equal(time.Unix(1725000000, 0)) {
		t.Fatalf("unexpected execution time: %s", got.ExecutedAt)
	}

	if got.Amount != "" {
		t.Fatalf("expected the amount to be left empty, got %q", got.Amount)
	}
}

// both constructors return an error for symmetry with the rest of the package,
// but neither can fail today — pin that so a future validation shows up here
func TestProtoStructConstructorsHaveNoErrorPathYet(t *testing.T) {
	if _, err := NewMarketOrderEntity(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := NewMarketHistoryEntity(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testLiquidityPool() *tradebinTypes.LiquidityPool {
	return &tradebinTypes.LiquidityPool{
		Id:           factoryDenom + "_" + nativeDenom,
		Base:         factoryDenom,
		Quote:        nativeDenom,
		LpDenom:      "ulp_" + factoryDenom + "_" + nativeDenom,
		Creator:      "bze1creator",
		Fee:          math.LegacyMustNewDecFromStr("0.003"),
		ReserveBase:  math.NewInt(1500000),
		ReserveQuote: math.NewInt(2500000),
	}
}

func TestNewMarketLiquidityDataEntity(t *testing.T) {
	pool := testLiquidityPool()

	got := NewMarketLiquidityDataEntity(pool)

	// the pool id doubles as the market id for pool-backed markets
	if got.MarketID != pool.Id {
		t.Fatalf("expected market id %q, got %q", pool.Id, got.MarketID)
	}

	if got.LpDenom != pool.LpDenom {
		t.Fatalf("unexpected lp denom: %q", got.LpDenom)
	}

	if got.Fee != "0.003000000000000000" {
		t.Fatalf("unexpected fee: %q", got.Fee)
	}

	if got.ReserveBase != "1500000" || got.ReserveQuote != "2500000" {
		t.Fatalf("unexpected reserves: %q/%q", got.ReserveBase, got.ReserveQuote)
	}
}

func TestNewMarketEntityFromLiquidityPool(t *testing.T) {
	pool := testLiquidityPool()
	before := time.Now()

	got := NewMarketEntityFromLiquidityPool(pool)

	// unlike NewMarketEntity the id comes straight from the pool, it is not
	// rebuilt from base and quote
	if got.MarketID != pool.Id {
		t.Fatalf("expected market id %q, got %q", pool.Id, got.MarketID)
	}

	if got.Base != factoryDenom || got.Quote != nativeDenom {
		t.Fatalf("unexpected denoms: %q/%q", got.Base, got.Quote)
	}

	if got.CreatedBy != "bze1creator" {
		t.Fatalf("unexpected creator: %q", got.CreatedBy)
	}

	if got.CreatedAt.Before(before) || got.CreatedAt.After(time.Now()) {
		t.Fatalf("expected CreatedAt to be stamped now, got %s", got.CreatedAt)
	}
}

func TestNewMarketHistoryFromSwap(t *testing.T) {
	executedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

	got := NewMarketHistoryFromSwap(&dto.SwapEventData{
		EventID:    77,
		PoolID:     "aaa_bbb",
		Creator:    "bze1creator",
		Input:      sdk.NewCoin("aaa", math.NewInt(2000000)),
		Output:     sdk.NewCoin("bbb", math.NewInt(4000000)),
		ExecutedAt: executedAt,
	})

	if got.MarketID != "aaa_bbb" {
		t.Fatalf("unexpected market id: %q", got.MarketID)
	}

	if !got.ExecutedAt.Equal(executedAt) {
		t.Fatalf("unexpected execution time: %s", got.ExecutedAt)
	}

	// a swap has no maker: the pool is the counterparty
	if got.Maker != "" {
		t.Fatalf("expected no maker, got %q", got.Maker)
	}

	if got.Taker != "bze1creator" {
		t.Fatalf("expected the swap creator as taker, got %q", got.Taker)
	}

	// amounts, price and order type are filled in by TypesConverter
	if got.Amount != "" || got.QuoteAmount != "" || got.Price != "" || got.OrderType != "" {
		t.Fatalf("expected the converted fields to be left empty, got %+v", got)
	}
}
