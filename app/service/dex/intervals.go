package dex

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/query"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/interval"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	"time"
)

type intervalStore interface {
	GetIntervalsBy(params *query.IntervalsParams) (query.IntervalsMap, error)
}

type Intervals struct {
	iRepo  intervalStore
	logger logrus.FieldLogger
	mRepo  ordersMarketRepo
}

func NewIntervals(iRepo intervalStore, logger logrus.FieldLogger, mRepo ordersMarketRepo) (*Intervals, error) {
	if iRepo == nil || logger == nil || mRepo == nil {
		return nil, internal.NewInvalidDependenciesErr("NewIntervalsService")
	}

	return &Intervals{
		iRepo:  iRepo,
		logger: logger,
		mRepo:  mRepo,
	}, nil
}

func (i *Intervals) GetIntervals(marketId string, length int, limit int) (result []entity.MarketHistoryInterval, err error) {
	l := i.logger.WithField("method", "GetIntervals")
	market, err := i.mRepo.GetMarket(marketId)
	if err != nil {
		return nil, err
	}

	if market == nil {
		return nil, fmt.Errorf("market not found: %s", marketId)
	}

	queryParams := i.getQueryParams(market, length, limit)
	entries, err := i.iRepo.GetIntervalsBy(queryParams)
	if err != nil {
		l.WithError(err).Error("failed to get intervals from repo")

		return nil, fmt.Errorf("failed to get intervals")
	}

	//if we found all required intervals then return them directly
	if len(entries) == limit {
		return entries.Elements(), nil
	}

	//if we didn't find all intervals needed we should fill the missing ones with 0 intervals
	intervalDuration := i.getIntervalDuration(length)
	nowStart, nowEnd := interval.GetTimestampInterval(time.Now().Unix(), interval.Length(length))
	for {
		if nowStart.Before(queryParams.StartAt) {
			break
		}

		entry, ok := entries[nowStart.Unix()]
		if ok {
			result = append(result, entry)
			nowStart, nowEnd = interval.GetTimestampInterval(
				nowStart.Add(-intervalDuration).Unix(),
				interval.Length(length),
			)

			continue
		}

		entry = entity.MarketHistoryInterval{
			MarketID:     marketId,
			Length:       length,
			StartAt:      nowStart,
			EndAt:        nowEnd,
			LowestPrice:  "0",
			OpenPrice:    "0",
			AveragePrice: "0",
			HighestPrice: "0",
			ClosePrice:   "0",
			BaseVolume:   "0",
			QuoteVolume:  "0",
		}

		result = append(result, entry)
		nowStart, nowEnd = interval.GetTimestampInterval(
			nowStart.Add(-intervalDuration).Unix(),
			interval.Length(length),
		)
	}

	return result, nil
}

func (i *Intervals) getQueryParams(market *entity.Market, length int, limit int) *query.IntervalsParams {
	//search only the intervals needed
	//use NOW - duration of all intervals as start at
	var startAt time.Time
	if limit > 0 {
		startAt = time.Now().Add(-time.Duration(limit) * i.getIntervalDuration(length))
	} else {
		startAt = market.CreatedAt
	}

	return &query.IntervalsParams{
		MarketId: market.MarketID,
		StartAt:  startAt,
		Limit:    limit,
		Length:   length,
	}
}

func (i *Intervals) getIntervalDuration(length int) time.Duration {
	return time.Duration(length) * time.Minute
}
