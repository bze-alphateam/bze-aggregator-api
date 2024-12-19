package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"time"
)

func NewMarketEntity(source *tradebinTypes.Market) *entity.Market {
	return &entity.Market{
		MarketID:  fmt.Sprintf("%s/%s", source.GetBase(), source.GetQuote()),
		Base:      source.GetBase(),
		Quote:     source.GetQuote(),
		CreatedBy: source.GetCreator(),
		CreatedAt: time.Now(),
	}
}

func NewMarketOrderEntity(source *tradebinTypes.AggregatedOrder) (*entity.MarketOrder, error) {
	return &entity.MarketOrder{
		MarketID:  source.GetMarketId(),
		OrderType: source.GetOrderType(),
		Price:     source.GetPrice(),
	}, nil
}

func NewMarketHistoryEntity(source *tradebinTypes.HistoryOrder) (*entity.MarketHistory, error) {
	return &entity.MarketHistory{
		MarketID:   source.GetMarketId(),
		OrderType:  source.GetOrderType(),
		Price:      source.GetPrice(),
		ExecutedAt: time.Unix(source.GetExecutedAt(), 0),
		Maker:      source.GetMaker(),
		Taker:      source.GetTaker(),
	}, nil
}
