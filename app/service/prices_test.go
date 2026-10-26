package service

import (
	"strings"
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

const (
	testNative = "ubze"
	testUsdc   = "ibc/usdc"
	testPoolId = "ibc/usdc_ubze"
	testLp     = "ulp_" + testPoolId
)

type fakePriceCache struct{ data map[string][]byte }

func (c *fakePriceCache) Get(key string) ([]byte, error) { return c.data[key], nil }

func (c *fakePriceCache) Set(key string, data []byte, _ time.Duration) error {
	c.data[key] = data
	return nil
}

type fakePriceProvider struct{ prices []dto.CoinPrice }

func (p *fakePriceProvider) GetDenominationsPrices() ([]dto.CoinPrice, error) {
	return p.prices, nil
}

type fakePriceMarketRepo struct{ markets entity.MarketsMap }

func (r *fakePriceMarketRepo) GetMarketsWithLastExecuted(int) ([]entity.MarketWithLastPrice, error) {
	return nil, nil
}

func (r *fakePriceMarketRepo) GetMarketsMap() (entity.MarketsMap, error) { return r.markets, nil }

type fakePriceLiquidityRepo struct {
	pools map[string]*entity.MarketLiquidityData
}

func (r *fakePriceLiquidityRepo) GetAllLiquidityPoolsIds() ([]string, error) { return nil, nil }

func (r *fakePriceLiquidityRepo) GetLiquidityDataByMarketId(marketId string) (*entity.MarketLiquidityData, error) {
	return r.pools[marketId], nil
}

type fakePriceHistoryRepo struct{}

func (fakePriceHistoryRepo) GetLastHistoryOrder(string) (*entity.MarketHistory, error) {
	return nil, nil
}

type fakePriceRegistry struct{}

func (fakePriceRegistry) GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error) {
	return &chain_registry.ChainRegistryAsset{
		Base:    denom,
		Display: "display" + denom,
		Symbol:  strings.ToUpper(denom),
		DenomUnits: []chain_registry.ChainRegistryAssetDenom{
			{Denom: denom, Exponent: 0},
			{Denom: "display" + denom, Exponent: 6},
		},
	}, nil
}

type fakePriceSupply struct{ supply map[string]string }

func (s *fakePriceSupply) GetUTotalSupply(denom string) (string, error) { return s.supply[denom], nil }

// newPoolPricesService wires a PricesService around a single USDC/BZE pool
// with the given reserves and LP supply. The external provider prices BZE at
// 0.002 USD, the fallback used when the internal markets yield no price.
func newPoolPricesService(t *testing.T, reserveBase, reserveQuote, lpSupply string) *PricesService {
	t.Helper()
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.PanicLevel)

	svc, err := NewPricesService(
		&fakePriceCache{data: map[string][]byte{}},
		&fakePriceProvider{prices: []dto.CoinPrice{{Denom: testNative, Price: 0.002, PriceDenom: "usd"}}},
		logger,
		&fakePriceMarketRepo{markets: entity.MarketsMap{
			testPoolId: {MarketID: testPoolId, Base: testUsdc, Quote: testNative},
		}},
		&fakePriceLiquidityRepo{pools: map[string]*entity.MarketLiquidityData{
			testPoolId: {MarketID: testPoolId, LpDenom: testLp, ReserveBase: reserveBase, ReserveQuote: reserveQuote},
		}},
		fakePriceHistoryRepo{},
		fakePriceRegistry{},
		&fakePriceSupply{supply: map[string]string{testLp: lpSupply}},
		testNative,
		testUsdc,
	)
	if err != nil {
		t.Fatalf("NewPricesService: %v", err)
	}

	return svc
}

// An emptied pool (reserves 0/0) used to panic with a division by zero while
// pricing BZE from it, which killed the API process from the tickers goroutines.
func TestCalculateInternalPrice_EmptiedPoolFallsBackToProvider(t *testing.T) {
	svc := newPoolPricesService(t, "0", "0", "0")

	price, err := svc.CalculateInternalPrice(testNative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 0.002 {
		t.Fatalf("BZE price = %v, want the provider fallback 0.002", price)
	}
}

func TestCalculateInternalPrice_EmptiedPoolLpHasNoPrice(t *testing.T) {
	svc := newPoolPricesService(t, "0", "0", "0")

	price, err := svc.CalculateInternalPrice(testLp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 0 {
		t.Fatalf("LP price = %v, want 0", price)
	}
}

func TestCalculateInternalPrice_ZeroLpSupplyHasNoPrice(t *testing.T) {
	svc := newPoolPricesService(t, "1000000", "500000000", "0")

	price, err := svc.CalculateInternalPrice(testLp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 0 {
		t.Fatalf("LP price = %v, want 0", price)
	}
}

func TestCalculateInternalPrice_PoolPrice(t *testing.T) {
	// 1 USDC against 500 BZE: BZE is worth 0.002 USDC.
	svc := newPoolPricesService(t, "1000000", "500000000", "1000000000000")

	price, err := svc.CalculateInternalPrice(testNative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 0.002 {
		t.Fatalf("BZE price = %v, want 0.002", price)
	}

	// 1 USDC + 500 BZE at 0.002 = 2 USD over 1 LP token.
	lpPrice, err := svc.CalculateInternalPrice(testLp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lpPrice != 2 {
		t.Fatalf("LP price = %v, want 2", lpPrice)
	}
}
