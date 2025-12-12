package sync

import (
	"fmt"
	"slices"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type blockTimeProvider interface {
	GetBlockTime(height int64) (time.Time, error)
}

type eventRepository interface {
	GetUnprocessedSwapEvents(limit int) ([]entity.Event, error)
	GetEventAttributes(eventID int64) ([]entity.EventAttribute, error)
	MarkEventAsProcessed(eventID int64) error
}

type marketHistoryRepository interface {
	SaveMarketHistory(items []*entity.MarketHistory) error
}

type SwapEventSync struct {
	eventRepo         eventRepository
	historyRepo       marketHistoryRepository
	logger            logrus.FieldLogger
	assetProvider     assetProvider
	locker            locker
	blockTimeProvider blockTimeProvider
}

func NewSwapEventSync(
	logger logrus.FieldLogger,
	eventRepo eventRepository,
	historyRepo marketHistoryRepository,
	provider assetProvider,
	l locker,
	blockTimeProvider blockTimeProvider,
) (*SwapEventSync, error) {
	if logger == nil || eventRepo == nil || historyRepo == nil || provider == nil || l == nil || blockTimeProvider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSwapEventSync")
	}

	return &SwapEventSync{
		eventRepo:         eventRepo,
		historyRepo:       historyRepo,
		logger:            logger.WithField("service", "SwapEventSync"),
		assetProvider:     provider,
		locker:            l,
		blockTimeProvider: blockTimeProvider,
	}, nil
}

func (s *SwapEventSync) SyncSwapEvents(batchSize int) (pools []string, err error) {
	s.locker.Lock(getSwapEventsLockKey())
	defer s.locker.Unlock(getSwapEventsLockKey())

	events, err := s.eventRepo.GetUnprocessedSwapEvents(batchSize)
	if err != nil {
		return pools, err
	}

	if len(events) == 0 {
		s.logger.Info("no unprocessed swap events found")
		return pools, nil
	}

	s.logger.Infof("processing %d swap events", len(events))

	for _, event := range events {
		poolId, err := s.processEvent(&event)
		if err != nil {
			s.logger.WithError(err).Errorf("error processing event %d, skipping", event.RowID)
			continue
		}

		if slices.Contains(pools, poolId) {
			continue
		}

		pools = append(pools, poolId)
	}

	s.logger.Infof("successfully processed %d swap events for %d pools", len(events), len(pools))
	return pools, nil
}

func (s *SwapEventSync) processEvent(event *entity.Event) (poolId string, err error) {
	// Get event attributes
	attributes, err := s.eventRepo.GetEventAttributes(event.RowID)
	if err != nil {
		return poolId, err
	}

	// Parse swap event data
	swapData, err := converter.ConvertEventToSwapData(event, attributes)
	if err != nil {
		return poolId, fmt.Errorf("error parsing swap event data: %w", err)
	}
	poolId = swapData.PoolID

	conv, err := converter.NewTypesConverter(s.assetProvider, swapData.GetBase().Denom, swapData.GetQuote().Denom)
	if err != nil {
		return poolId, fmt.Errorf("error creating types converter: %w", err)
	}

	// Create market history entry
	historyEntry, err := conv.SwapDataToHistoryEntity(*swapData)
	if err != nil {
		return poolId, fmt.Errorf("error creating market history entry: %w", err)
	}

	historyEntry.ExecutedAt, err = s.blockTimeProvider.GetBlockTime(event.BlockHeight)
	if err != nil {
		return poolId, fmt.Errorf("error getting block time: %w", err)
	}

	// Save market history
	err = s.historyRepo.SaveMarketHistory([]*entity.MarketHistory{historyEntry})
	if err != nil {
		return poolId, fmt.Errorf("error saving market history: %w", err)
	}

	// Mark event as processed
	err = s.eventRepo.MarkEventAsProcessed(event.RowID)
	if err != nil {
		return poolId, fmt.Errorf("error marking event as processed: %w", err)
	}

	s.logger.Infof("processed swap event %d for pool %s", event.RowID, swapData.PoolID)
	return poolId, nil
}
