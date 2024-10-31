package converter

import (
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/interval"
)

func IntervalMapToEntities(source *interval.Map) []*entity.MarketHistoryInterval {
	intervals := source.GetIntervals()
	result := make([]*entity.MarketHistoryInterval, len(intervals))
	for i, src := range intervals {
		result[i] = &entity.MarketHistoryInterval{
			MarketID:     source.MarketId,
			Length:       int(src.Duration),
			StartAt:      src.Start,
			EndAt:        src.End,
			LowestPrice:  TrimAmountTrailingZeros(src.LowestPrice.String()),
			HighestPrice: TrimAmountTrailingZeros(src.HighestPrice.String()),
			OpenPrice:    TrimAmountTrailingZeros(src.OpenPrice.String()),
			ClosePrice:   TrimAmountTrailingZeros(src.ClosePrice.String()),
			AveragePrice: TrimAmountTrailingZeros(src.AveragePrice.String()),
			BaseVolume:   TrimAmountTrailingZeros(src.BaseVolume.String()),
			QuoteVolume:  TrimAmountTrailingZeros(src.BaseVolume.String()),
		}
	}

	return result
}
