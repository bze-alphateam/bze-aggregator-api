package converter

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"strconv"
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
	amtInt, err := strconv.Atoi(source.GetAmount())
	if err != nil {
		return nil, err
	}

	return &entity.MarketOrder{
		MarketID:  source.GetMarketId(),
		OrderType: source.GetOrderType(),
		Amount:    uint64(amtInt),
		Price:     source.GetPrice(),
	}, nil
}
