package sync

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/interval"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type histStorage interface {
	GetByExecutedAt(marketId string, executedAt time.Time) ([]entity.MarketHistory, error)
	GetOldestNotAddedToInterval(marketId string) (*entity.MarketHistory, error)
	MarkAsAddedToInterval(ids []int) error
}

type intervalStorage interface {
	Save([]*entity.MarketHistoryInterval) error
}

type IntervalSync struct {
	logger          logrus.FieldLogger
	hist            histStorage
	locker          locker
	intervalStorage intervalStorage
}

func NewIntervalSync(logger logrus.FieldLogger, histStorage histStorage, l locker, intervalStorage intervalStorage) (*IntervalSync, error) {
	if logger == nil || histStorage == nil || l == nil || intervalStorage == nil {
		return nil, internal.NewInvalidDependenciesErr("NewIntervalSync")
	}

	return &IntervalSync{
		logger:          logger,
		hist:            histStorage,
		locker:          l,
		intervalStorage: intervalStorage,
	}, nil
}

// SyncIntervals - queries for last intervals synced for each configured duration and tries to fill them from history
func (i *IntervalSync) SyncIntervals(market *tradebinTypes.Market) error {
	marketId := converter.GetMarketId(market.GetBase(), market.GetQuote())

	i.locker.Lock(getIntervalLockKey(marketId))
	defer i.locker.Unlock(getIntervalLockKey(marketId))

	l := i.logger.WithField("market_id", marketId)
	l.Info("preparing to sync market intervals")

	oldest, err := i.hist.GetOldestNotAddedToInterval(marketId)
	if err != nil {
		return fmt.Errorf("error getting oldest not-added to interval: %s", err.Error())
	}

	if oldest == nil {
		return fmt.Errorf("no orders found to add to intervals")
	}

	timestampToSync, _ := interval.GetTimestampInterval(oldest.ExecutedAt.Unix(), interval.GetBiggestDuration())
	orders, err := i.hist.GetByExecutedAt(marketId, timestampToSync)
	if err != nil {
		return fmt.Errorf("error getting orders from history: %s", err.Error())
	}

	iMap := interval.NewIntervalsMap(marketId)
	var added []int
	for _, order := range orders {
		iMap.AddOrder(&order)
		added = append(added, order.ID)
	}

	toSave := converter.IntervalMapToEntities(iMap)
	if len(toSave) == 0 {
		return fmt.Errorf("we had orders to save but nothing found in the intervals map")
	}

	toSaveBatches := converter.SplitIntervalsSlice(toSave, 1000)
	for _, entities := range toSaveBatches {
		//wg.Add(1)
		//go func() {
		//	defer wg.Done()
		err = i.intervalStorage.Save(entities)
		if err != nil {
			l.WithError(err).Error("could not save intervals batch")
		}
		//}()
	}

	batches := converter.SplitIntSlice(added, 1000)
	wg := sync.WaitGroup{}
	for _, batch := range batches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = i.hist.MarkAsAddedToInterval(batch)
			if err != nil {
				l.WithError(err).Error("could not mark intervals as added")
			}
		}()
	}

	l.Info("waiting for history orders to be marked as done and intervals saved")
	wg.Wait()

	return nil
}
