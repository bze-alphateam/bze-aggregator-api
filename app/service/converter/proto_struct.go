package converter

import (
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
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

func NewMarketLiquidityDataEntity(source *tradebinTypes.LiquidityPool) *entity.MarketLiquidityData {
	return &entity.MarketLiquidityData{
		MarketID:     source.Id,
		LpDenom:      source.GetLpDenom(),
		Fee:          source.Fee.String(),
		ReserveBase:  source.ReserveBase.String(),
		ReserveQuote: source.ReserveQuote.String(),
	}
}

func NewMarketEntityFromLiquidityPool(source *tradebinTypes.LiquidityPool) *entity.Market {
	return &entity.Market{
		MarketID:  source.GetId(),
		Base:      source.GetBase(),
		Quote:     source.GetQuote(),
		CreatedBy: source.GetCreator(),
		CreatedAt: time.Now(),
	}
}

func NewMarketHistoryFromSwap(source *dto.SwapEventData) *entity.MarketHistory {
	return &entity.MarketHistory{
		MarketID:   source.PoolID,
		ExecutedAt: source.ExecutedAt,
		Maker:      "",
		Taker:      source.Creator,
	}
}
