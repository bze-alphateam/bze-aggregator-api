package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"strconv"
	"time"
)

func NewMarketEntity(source *tradebinTypes.Market) *entity.Market {
	return &entity.Market{
		MarketID:  fmt.Sprintf("%s/%s", source.GetBase(), source.GetQuote()),
		Base:      source.GetBase(),
		Quote:     source.GetQuote(),
		CreatedBy: source.GetCreator(),
	}
}

func NewMarketOrderEntity(source *tradebinTypes.AggregatedOrder) (*entity.MarketOrder, error) {
	amtInt, err := strconv.ParseUint(source.GetAmount(), 10, 64)
	if err != nil {
		return nil, err
	}

	qAmount, err := GetQuoteAmount(amtInt, source.GetPrice())
	if err != nil {
		return nil, err
	}

	//TODO: uPrice transformation
	return &entity.MarketOrder{
		MarketID:    source.GetMarketId(),
		OrderType:   source.GetOrderType(),
		Amount:      amtInt,
		Price:       source.GetPrice(),
		QuoteAmount: qAmount,
	}, nil
}

func NewMarketHistoryEntity(source *tradebinTypes.HistoryOrder) (*entity.MarketHistory, error) {
	amtInt, err := strconv.ParseUint(source.GetAmount(), 10, 64)
	if err != nil {
		return nil, err
	}

	//TODO: uPrice transformation
	qAmount, err := GetQuoteAmount(amtInt, source.GetPrice())
	if err != nil {
		return nil, err
	}

	return &entity.MarketHistory{
		ID:          0,
		MarketID:    source.GetMarketId(),
		OrderType:   source.GetOrderType(),
		Amount:      amtInt,
		Price:       source.GetPrice(),
		ExecutedAt:  time.Unix(source.GetExecutedAt(), 0),
		Maker:       source.GetMaker(),
		Taker:       source.GetTaker(),
		QuoteAmount: qAmount,
	}, nil
}
