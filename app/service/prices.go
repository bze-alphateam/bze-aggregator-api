package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

const (
	pricesCache       = "prices:all"
	pricesCacheBackup = "prices:all:backup"

	priceCacheExpireSeconds       = 180
	priceBackupCacheExpireSeconds = 60 * 60 * 24 //1 day

	internalPriceCacheTTL = 60 * time.Second // Cache internal prices for 1 minute
)

type PriceProvider interface {
	GetDenominationsPrices() ([]dto.CoinPrice, error)
}

type MarketRepo interface {
	GetMarketsWithLastExecuted(hours int) ([]entity.MarketWithLastPrice, error)
	GetMarketsMap() (entity.MarketsMap, error)
}

type MarketLiquidityRepo interface {
	GetAllLiquidityPoolsIds() ([]string, error)
	GetLiquidityDataByMarketId(marketId string) (*entity.MarketLiquidityData, error)
}

type MarketHistoryRepo interface {
	GetLastHistoryOrder(marketId string) (*entity.MarketHistory, error)
}

type ChainRegistryProvider interface {
	GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error)
}

type SupplyProvider interface {
	GetUTotalSupply(denom string) (string, error)
}

type PricesService struct {
	cache          Cache
	dataProvider   PriceProvider
	logger         logrus.FieldLogger
	marketRepo     MarketRepo
	liquidityRepo  MarketLiquidityRepo
	historyRepo    MarketHistoryRepo
	chainRegistry  ChainRegistryProvider
	supplyProvider SupplyProvider
	nativeDenom    string
	usdcDenom      string
}

func NewPricesService(
	cache Cache,
	dataProvider PriceProvider,
	logger logrus.FieldLogger,
	marketRepo MarketRepo,
	liquidityRepo MarketLiquidityRepo,
	historyRepo MarketHistoryRepo,
	chainRegistry ChainRegistryProvider,
	supplyProvider SupplyProvider,
	nativeDenom string,
	usdcDenom string,
) (*PricesService, error) {
	if dataProvider == nil || cache == nil || logger == nil ||
		marketRepo == nil || liquidityRepo == nil || historyRepo == nil ||
		chainRegistry == nil || supplyProvider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewPricesService")
	}

	return &PricesService{
		cache:          cache,
		dataProvider:   dataProvider,
		logger:         logger.WithField("service", "Service.Prices"),
		marketRepo:     marketRepo,
		liquidityRepo:  liquidityRepo,
		historyRepo:    historyRepo,
		chainRegistry:  chainRegistry,
		supplyProvider: supplyProvider,
		nativeDenom:    nativeDenom,
		usdcDenom:      usdcDenom,
	}, nil
}

func (p *PricesService) GetPrices() []dto.CoinPrice {
	cacheValue := p.getPricesFromCache(pricesCache)
	if cacheValue != nil {
		return cacheValue
	}

	prices := p.getPricesFromProvider()
	if prices == nil {
		//return backup
		return p.getPricesFromCache(pricesCacheBackup)
	}

	p.cachePrices(prices)

	return prices
}

func (p *PricesService) cachePrices(prices []dto.CoinPrice) {
	encoded, err := json.Marshal(prices)
	if err != nil {
		p.logger.Errorf("failed to marshal prices in order to cache them: %v", err)

		return
	}

	err = p.cache.Set(pricesCache, encoded, time.Duration(priceCacheExpireSeconds)*time.Second)
	if err != nil {
		p.logger.Errorf("failed to cache prices: %v", err)
	}

	err = p.cache.Set(pricesCacheBackup, encoded, time.Duration(priceBackupCacheExpireSeconds)*time.Second)
	if err != nil {
		p.logger.Errorf("failed to cache prices for backup: %v", err)
	}
}

func (p *PricesService) getBackupPrices() []dto.CoinPrice {
	return p.getPricesFromCache(pricesCacheBackup)
}

func (p *PricesService) getPricesFromCache(key string) []dto.CoinPrice {
	cacheValue, err := p.cache.Get(key)
	if err != nil {
		p.logger.Errorf("failed to get prices from cache: %v", err)

		return nil
	}

	if cacheValue != nil {
		var prices []dto.CoinPrice
		err = json.Unmarshal(cacheValue, &prices)
		if err != nil {
			p.logger.Errorf("failed to unmarshal prices from cache: %v", err)
		} else {
			return prices
		}
	}

	return nil
}

func (p *PricesService) getPricesFromProvider() []dto.CoinPrice {
	prices, err := p.dataProvider.GetDenominationsPrices()
	if err != nil {
		p.logger.Errorf("failed to get prices from provider: %v", err)

		return nil
	}

	return prices
}

// CalculateInternalPrice calculates the USD price for a given denom using internal exchange data
func (p *PricesService) CalculateInternalPrice(denom string) (float64, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("internal_price:%s", denom)
	cachedData, err := p.cache.Get(cacheKey)
	if err == nil && cachedData != nil {
		if price, err := strconv.ParseFloat(string(cachedData), 64); err == nil {
			return price, nil
		}
	}

	// Calculate price
	var price float64
	var calcErr error

	// USDC is always $1.00
	if denom == p.usdcDenom {
		price = 1.0
	} else if converter.IsLpDenom(denom) {
		// Handle LP tokens
		price, calcErr = p.calculateLpPrice(denom)
	} else if denom == p.nativeDenom {
		// Handle native token (BZE) - get BZE/USDC price with fallback
		price, calcErr = p.getLastPrice(p.nativeDenom, p.usdcDenom, func() (float64, error) {
			// Fallback to external price provider
			prices, err := p.dataProvider.GetDenominationsPrices()
			if err != nil {
				return 0, err
			}
			for _, cp := range prices {
				if cp.Denom == p.nativeDenom && cp.PriceDenom == "usd" {
					return cp.Price, nil
				}
			}
			return 0, fmt.Errorf("BZE price not found in provider")
		})
		if calcErr != nil {
			p.logger.Errorf("failed to get BZE price: %v", calcErr)
			price = 0.0
			calcErr = nil
		}
	} else {
		// For other assets, try to get price in USDC and BZE
		priceInUsd, _ := p.getLastPrice(denom, p.usdcDenom, nil)
		priceInBze, _ := p.getLastPrice(denom, p.nativeDenom, nil)

		if priceInBze <= 0 {
			// No BZE market -> use USD price
			price = priceInUsd
		} else {
			// Get BZE/USD price
			bzeUsdPrice, err := p.CalculateInternalPrice(p.nativeDenom)
			if err != nil || bzeUsdPrice <= 0 {
				price = priceInUsd
			} else {
				priceInUsdFromBze := priceInBze * bzeUsdPrice

				if priceInUsd <= 0 {
					// No USD market -> use BZE-derived price
					price = priceInUsdFromBze
				} else {
					// Both markets exist -> return average
					price = (priceInUsd + priceInUsdFromBze) / 2
				}
			}
		}
	}

	// Cache the result
	priceStr := strconv.FormatFloat(price, 'f', -1, 64)
	_ = p.cache.Set(cacheKey, []byte(priceStr), internalPriceCacheTTL)

	return price, calcErr
}

// calculateLpPrice calculates the price of an LP token based on reserves
func (p *PricesService) calculateLpPrice(lpDenom string) (float64, error) {
	poolId := converter.PoolIdFromPoolDenom(lpDenom)

	// Get pool liquidity data
	liquidityData, err := p.getLiquidityData(poolId)
	if err != nil || liquidityData == nil {
		p.logger.Errorf("failed to get liquidity data for pool %s: %v", poolId, err)
		return 0.0, fmt.Errorf("liquidity data not found for pool %s", poolId)
	}

	// Get market to determine base and quote denoms
	market, err := p.marketRepo.GetMarketsMap()
	if err != nil {
		return 0.0, err
	}

	marketData := market.Get(poolId)
	if marketData == nil {
		return 0.0, fmt.Errorf("market not found for pool %s", poolId)
	}

	baseDenom := marketData.Base
	quoteDenom := marketData.Quote

	// Recursively get prices for base and quote assets
	basePrice, err := p.CalculateInternalPrice(baseDenom)
	if err != nil || basePrice <= 0 {
		p.logger.Errorf("failed to get base price for %s: %v", baseDenom, err)
		return 0.0, nil
	}

	quotePrice, err := p.CalculateInternalPrice(quoteDenom)
	if err != nil || quotePrice <= 0 {
		p.logger.Errorf("failed to get quote price for %s: %v", quoteDenom, err)
		return 0.0, nil
	}

	// Get asset decimals
	baseAsset, err := p.chainRegistry.GetAssetDetails(baseDenom)
	if err != nil || baseAsset == nil {
		p.logger.Errorf("failed to get base asset details for %s: %v", baseDenom, err)
		return 0.0, nil
	}
	baseDisplayDenom := baseAsset.GetDisplayDenomUnit()
	if baseDisplayDenom == nil {
		p.logger.Warnf("no display denom for base asset %s, skipping LP price calculation", baseDenom)
		return 0.0, nil
	}

	quoteAsset, err := p.chainRegistry.GetAssetDetails(quoteDenom)
	if err != nil || quoteAsset == nil {
		p.logger.Errorf("failed to get quote asset details for %s: %v", quoteDenom, err)
		return 0.0, nil
	}
	quoteDisplayDenom := quoteAsset.GetDisplayDenomUnit()
	if quoteDisplayDenom == nil {
		p.logger.Warnf("no display denom for quote asset %s, skipping LP price calculation", quoteDenom)
		return 0.0, nil
	}

	// Get LP total supply
	lpSupply, err := p.supplyProvider.GetUTotalSupply(lpDenom)
	if err != nil {
		p.logger.Errorf("failed to get LP supply for %s: %v", lpDenom, err)
		return 0.0, err
	}

	// Convert reserves and supply to LegacyDec
	reserveBase, err := sdkmath.LegacyNewDecFromStr(liquidityData.ReserveBase)
	if err != nil {
		return 0.0, err
	}

	reserveQuote, err := sdkmath.LegacyNewDecFromStr(liquidityData.ReserveQuote)
	if err != nil {
		return 0.0, err
	}

	lpTotalSupply, err := sdkmath.LegacyNewDecFromStr(lpSupply)
	if err != nil {
		return 0.0, err
	}

	// Calculate total value in pool
	// baseValueUSD = basePrice * (reserveBase / 10^baseDecimals)
	// quoteValueUSD = quotePrice * (reserveQuote / 10^quoteDecimals)
	basePriceDec := sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("%f", basePrice))
	quotePriceDec := sdkmath.LegacyMustNewDecFromStr(fmt.Sprintf("%f", quotePrice))

	baseDecimalsDivisor := sdkmath.LegacyNewDec(10).Power(uint64(baseDisplayDenom.Exponent))
	quoteDecimalsDivisor := sdkmath.LegacyNewDec(10).Power(uint64(quoteDisplayDenom.Exponent))
	lpDecimalsDivisor := sdkmath.LegacyNewDec(10).Power(uint64(converter.GetLpAssetDecimals()))

	baseValueUsd := basePriceDec.Mul(reserveBase).Quo(baseDecimalsDivisor)
	quoteValueUsd := quotePriceDec.Mul(reserveQuote).Quo(quoteDecimalsDivisor)
	totalValueUsd := baseValueUsd.Add(quoteValueUsd)

	// LP token price = totalValueUSD / (lpTotalSupply / 10^12)
	lpPrice := totalValueUsd.Quo(lpTotalSupply.Quo(lpDecimalsDivisor))

	priceFloat, err := lpPrice.Float64()
	if err != nil {
		return 0.0, err
	}

	return priceFloat, nil
}

// getLastPrice attempts to get the last price for a base/quote pair from various sources
func (p *PricesService) getLastPrice(base, quote string, fallback func() (float64, error)) (float64, error) {
	// Try to get price from liquidity pool first
	poolId := converter.CreatePoolId(base, quote)
	liquidityData, err := p.getLiquidityData(poolId)
	if err == nil && liquidityData != nil {
		price, err := p.calculatePoolPrice(base, liquidityData)
		if err == nil && price > 0 {
			return price, nil
		}
	}

	// Try to get price from market data (last 24h)
	marketId := converter.GetMarketId(base, quote)
	marketsWithPrice, err := p.marketRepo.GetMarketsWithLastExecuted(24)
	if err == nil {
		for _, m := range marketsWithPrice {
			if m.MarketID == marketId && m.LastPrice.Valid {
				priceDec, err := sdkmath.LegacyNewDecFromStr(m.LastPrice.String)
				if err == nil {
					priceFloat, _ := priceDec.Float64()
					if priceFloat > 0 {
						return priceFloat, nil
					}
				}
			}
		}
	}

	// Try to get price from trade history
	history, err := p.historyRepo.GetLastHistoryOrder(marketId)
	if err == nil && history != nil && history.Price != "" {
		priceDec, err := sdkmath.LegacyNewDecFromStr(history.Price)
		if err == nil {
			priceFloat, _ := priceDec.Float64()
			if priceFloat > 0 {
				return priceFloat, nil
			}
		}
	}

	// Use fallback if provided
	if fallback != nil {
		return fallback()
	}

	return 0.0, nil
}

// calculatePoolPrice calculates the price of base asset in terms of quote from pool reserves
func (p *PricesService) calculatePoolPrice(baseDenom string, liquidityData *entity.MarketLiquidityData) (float64, error) {
	// Get market to determine which denom is base and which is quote
	market, err := p.marketRepo.GetMarketsMap()
	if err != nil {
		return 0.0, err
	}

	marketData := market.Get(liquidityData.MarketID)
	if marketData == nil {
		return 0.0, fmt.Errorf("market not found")
	}

	// Get asset details to handle decimal differences
	baseAsset, err := p.chainRegistry.GetAssetDetails(marketData.Base)
	if err != nil || baseAsset == nil {
		p.logger.Warnf("asset %s not found in chain registry, skipping pool price calculation", marketData.Base)
		return 0.0, nil
	}

	quoteAsset, err := p.chainRegistry.GetAssetDetails(marketData.Quote)
	if err != nil || quoteAsset == nil {
		p.logger.Warnf("asset %s not found in chain registry, skipping pool price calculation", marketData.Quote)
		return 0.0, nil
	}

	reserveBase, err := sdkmath.LegacyNewDecFromStr(liquidityData.ReserveBase)
	if err != nil {
		return 0.0, err
	}

	reserveQuote, err := sdkmath.LegacyNewDecFromStr(liquidityData.ReserveQuote)
	if err != nil {
		return 0.0, err
	}

	// Calculate raw price (price of base in terms of quote)
	var rawPrice sdkmath.LegacyDec
	if baseDenom == marketData.Base {
		// Price of base in terms of quote = reserveQuote / reserveBase
		rawPrice = reserveQuote.Quo(reserveBase)
	} else {
		// Price of quote in terms of base = reserveBase / reserveQuote
		rawPrice = reserveBase.Quo(reserveQuote)
		// If we're pricing the quote asset, we need to swap the assets for correct adjustment
		baseAsset, quoteAsset = quoteAsset, baseAsset
	}

	// Use existing converter to adjust for decimal differences
	_, adjustedPrice, err := converter.UPriceToPrice(baseAsset, quoteAsset, rawPrice.String())
	if err != nil {
		return 0.0, fmt.Errorf("failed to adjust price for decimals: %w", err)
	}

	return adjustedPrice, nil
}

// getLiquidityData retrieves liquidity data for a given pool ID
func (p *PricesService) getLiquidityData(poolId string) (*entity.MarketLiquidityData, error) {
	return p.liquidityRepo.GetLiquidityDataByMarketId(poolId)
}
