package converter

import (
	"fmt"
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

func NewTypesConverter(provider assetProvider, market *types.Market) (*TypesConverter, error) {
	bAsset, err := provider.GetAssetDetails(market.GetBase())
	if err != nil {
		return nil, err
	}

	if bAsset == nil {
		return nil, fmt.Errorf("base asset not found")
	}

	qAsset, err := provider.GetAssetDetails(market.GetQuote())
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

func (tc *TypesConverter) HistoryOrderToHistoryEntity(source *types.HistoryOrder) (*entity.MarketHistory, error) {
	ent, err := NewMarketHistoryEntity(source)
	if err != nil {
		return nil, err
	}

	ent.Price, err = UPriceToPrice(tc.base, tc.quote, source.Price)
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

	ent.Price, err = UPriceToPrice(tc.base, tc.quote, ent.Price)
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
