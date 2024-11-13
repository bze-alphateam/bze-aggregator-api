package dex

import (
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/response"
	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/bze-alphateam/bze/x/tradebin/types"
	"github.com/sirupsen/logrus"
	"time"
)

type historyRepo interface {
	GetHistoryBy(marketId, orderType string, limit int, startAt, endAt *time.Time) ([]entity.MarketHistory, error)
}

type HistoryService struct {
	logger      logrus.FieldLogger
	historyRepo historyRepo
}

func NewHistoryService(logger logrus.FieldLogger, historyRepo historyRepo) (*HistoryService, error) {
	if logger == nil || historyRepo == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHistoryService")
	}

	return &HistoryService{
		logger:      logger,
		historyRepo: historyRepo,
	}, nil
}

func (h *HistoryService) GetHistory(params *request.HistoryParams) ([]response.HistoryTrade, error) {
	hist, err := h.getHistory(params)
	if err != nil {
		return nil, err
	}

	var result []response.HistoryTrade
	for _, order := range hist {
		tr := response.HistoryTrade{
			OrderId:     order.ID,
			Price:       order.Price,
			BaseVolume:  order.Amount,
			QuoteVolume: order.QuoteAmount,
			ExecutedAt:  fmt.Sprintf("%d", order.ExecutedAt.UnixMilli()),
			OrderType:   order.OrderType,
		}

		result = append(result, tr)
	}

	return result, nil
}

func (h *HistoryService) GetCoingeckoHistory(params *request.HistoryParams) (*response.CoingeckoHistory, error) {
	hist, err := h.getHistory(params)
	if err != nil {
		return nil, err
	}

	result := response.CoingeckoHistory{}
	for _, order := range hist {
		tr := response.CoingeckoHistoryTrade{
			OrderId:     order.ID,
			Price:       order.Price,
			BaseVolume:  order.Amount,
			QuoteVolume: order.QuoteAmount,
			ExecutedAt:  fmt.Sprintf("%d", order.ExecutedAt.UnixMilli()),
			OrderType:   order.OrderType,
		}

		if order.OrderType == types.OrderTypeBuy {
			result.Buy = append(result.Buy, tr)
		} else {
			result.Sell = append(result.Sell, tr)
		}
	}

	return &result, nil
}

func (h *HistoryService) getHistory(params *request.HistoryParams) ([]entity.MarketHistory, error) {
	if params.StartTime > 0 && params.EndTime > params.StartTime {
		startAt := converter.MillisecondsToTime(params.StartTime)
		endAt := converter.MillisecondsToTime(params.EndTime)

		return h.historyRepo.GetHistoryBy(params.MustGetMarketId(), params.OrderType, params.Limit, &startAt, &endAt)
	}

	return h.historyRepo.GetHistoryBy(params.MustGetMarketId(), params.OrderType, params.Limit, nil, nil)
}
