package sync

import (
	"fmt"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
)

type eventRepository interface {
	GetUnprocessedSwapEvents(limit int) ([]entity.Event, error)
	GetEventAttributes(eventID int64) ([]entity.EventAttribute, error)
	MarkEventAsProcessed(eventID int64) error
}

type marketHistoryRepository interface {
	SaveMarketHistory(items []*entity.MarketHistory) error
}

type SwapEventSync struct {
	eventRepo     eventRepository
	historyRepo   marketHistoryRepository
	logger        logrus.FieldLogger
	assetProvider assetProvider
}

func NewSwapEventSync(logger logrus.FieldLogger, eventRepo eventRepository, historyRepo marketHistoryRepository, provider assetProvider) (*SwapEventSync, error) {
	if logger == nil || eventRepo == nil || historyRepo == nil || provider == nil {
		return nil, internal.NewInvalidDependenciesErr("NewSwapEventSync")
	}

	return &SwapEventSync{
		eventRepo:     eventRepo,
		historyRepo:   historyRepo,
		logger:        logger.WithField("service", "SwapEventSync"),
		assetProvider: provider,
	}, nil
}

func (s *SwapEventSync) SyncSwapEvents(batchSize int) (int, error) {
	events, err := s.eventRepo.GetUnprocessedSwapEvents(batchSize)
	if err != nil {
		return 0, err
	}

	if len(events) == 0 {
		s.logger.Info("no unprocessed swap events found")
		return 0, nil
	}

	s.logger.Infof("processing %d swap events", len(events))

	processedCount := 0
	for _, event := range events {
		err := s.processEvent(&event)
		if err != nil {
			s.logger.WithError(err).Errorf("error processing event %d, skipping", event.RowID)
			continue
		}

		processedCount++
	}

	s.logger.Infof("successfully processed %d out of %d swap events", processedCount, len(events))
	return processedCount, nil
}

func (s *SwapEventSync) processEvent(event *entity.Event) error {
	// Get event attributes
	attributes, err := s.eventRepo.GetEventAttributes(event.RowID)
	if err != nil {
		return err
	}

	// Parse swap event data
	swapData, err := converter.ConvertEventToSwapData(event, attributes)
	if err != nil {
		return fmt.Errorf("error parsing swap event data: %w", err)
	}

	conv, err := converter.NewTypesConverter(s.assetProvider, swapData.GetBase().Denom, swapData.GetQuote().Denom)
	if err != nil {
		return fmt.Errorf("error creating types converter: %w", err)
	}

	// Create market history entry
	historyEntry, err := conv.SwapDataToHistoryEntity(*swapData)
	if err != nil {
		return fmt.Errorf("error creating market history entry: %w", err)
	}

	// Save market history
	err = s.historyRepo.SaveMarketHistory([]*entity.MarketHistory{historyEntry})
	if err != nil {
		return fmt.Errorf("error saving market history: %w", err)
	}

	// Mark event as processed
	err = s.eventRepo.MarkEventAsProcessed(event.RowID)
	if err != nil {
		return fmt.Errorf("error marking event as processed: %w", err)
	}

	s.logger.Infof("processed swap event %d for pool %s", event.RowID, swapData.PoolID)
	return nil
}
