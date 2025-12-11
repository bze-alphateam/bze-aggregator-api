package converter

import (
	"fmt"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze/x/tradebin/types"
)

type assetProvider interface {
	GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error)
}

type TypesConverter struct {
	base  *chain_registry.ChainRegistryAsset
	quote *chain_registry.ChainRegistryAsset
}

func NewTypesConverter(provider assetProvider, base, quote string) (*TypesConverter, error) {
	bAsset, err := provider.GetAssetDetails(base)
	if err != nil {
		return nil, err
	}

	if bAsset == nil {
		return nil, fmt.Errorf("base asset not found")
	}

	qAsset, err := provider.GetAssetDetails(quote)
	if err != nil {
		return nil, err
	}

	if qAsset == nil {
		return nil, fmt.Errorf("quote asset not found")
	}

	return &TypesConverter{
		base:  bAsset,
		quote: qAsset,
	}, nil
}

func (tc *TypesConverter) SwapDataToHistoryEntity(source dto.SwapEventData) (*entity.MarketHistory, error) {
	ent := NewMarketHistoryFromSwap(&source)
	base := source.GetBase()
	quote := source.GetQuote()

	// Determine order type based on input denom
	// If input is base -> sell, if input is quote -> buy
	if source.Input.Denom == base.Denom {
		ent.OrderType = "sell"
	} else {
		// Buying base with quote
		ent.OrderType = "buy"
	}

	var err error
	ent.Amount, err = UAmountToAmount(tc.base, base.Amount.String())
	if err != nil {
		return nil, err
	}

	ent.QuoteAmount, err = UAmountToAmount(tc.quote, quote.Amount.String())
	if err != nil {
		return nil, err
	}

	// Convert to decimal for price calculation
	baseDec := math.LegacyNewDecFromInt(base.Amount)
	quoteDec := math.LegacyNewDecFromInt(quote.Amount)

	if baseDec.IsZero() {
		return nil, fmt.Errorf("base amount is zero")
	}

	priceDec := quoteDec.Quo(baseDec)
	ent.Price, _, err = UPriceToPrice(tc.base, tc.quote, priceDec.String())
	if err != nil {
		return nil, err
	}

	return ent, nil
}

func (tc *TypesConverter) HistoryOrderToHistoryEntity(source *types.HistoryOrder) (*entity.MarketHistory, error) {
	ent, err := NewMarketHistoryEntity(source)
	if err != nil {
		return nil, err
	}

	ent.Price, _, err = UPriceToPrice(tc.base, tc.quote, source.Price)
	if err != nil {
		return nil, err
	}

	ent.Amount, err = UAmountToAmount(tc.base, source.Amount)
	if err != nil {
		return nil, err
	}

	ent.QuoteAmount = GetQuoteAmount(ent.Amount, ent.Price, tc.quote)

	return ent, nil
}

func (tc *TypesConverter) AggregatedOrderToOrderEntity(source *types.AggregatedOrder) (*entity.MarketOrder, error) {
	ent, err := NewMarketOrderEntity(source)
	if err != nil {
		return nil, err
	}

	ent.Price, ent.PriceDec, err = UPriceToPrice(tc.base, tc.quote, ent.Price)
	if err != nil {
		return nil, err
	}

	ent.Amount, err = UAmountToAmount(tc.base, source.Amount)
	if err != nil {
		return nil, err
	}

	ent.QuoteAmount = GetQuoteAmount(ent.Amount, ent.Price, tc.quote)

	return ent, nil
}
