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
)

type historyRepo interface {
	GetHistoryBy(params request.HistoryParams) ([]entity.MarketHistory, error)
	GetAddressSwapHistory(address string) ([]entity.MarketHistory, error)
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
		logger:      logger.WithField("service", "Dex.HistoryService"),
		historyRepo: historyRepo,
	}, nil
}

func (h *HistoryService) GetHistory(params *request.HistoryParams) ([]response.HistoryTrade, error) {
	hist, err := h.historyRepo.GetHistoryBy(*params)
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
			Maker:       order.Maker,
			Taker:       order.Taker,
		}

		result = append(result, tr)
	}

	return result, nil
}

func (h *HistoryService) GetCoingeckoHistory(params *request.HistoryParams) (*response.CoingeckoHistory, error) {
	hist, err := h.historyRepo.GetHistoryBy(*params)
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

func (h *HistoryService) GetAddressSwapHistory(address string) ([]response.HistoryTrade, error) {
	hist, err := h.historyRepo.GetAddressSwapHistory(address)
	if err != nil {
		return nil, err
	}

	result := make([]response.HistoryTrade, 0)
	for _, order := range hist {
		base, denom, err := converter.PoolIdToDenoms(order.MarketID)
		if err != nil {
			h.logger.WithError(err).Error("error converting pool id to denoms")
			continue
		}

		tr := response.HistoryTrade{
			OrderId:     order.ID,
			PoolId:      order.MarketID,
			Price:       order.Price,
			BaseVolume:  order.Amount,
			QuoteVolume: order.QuoteAmount,
			ExecutedAt:  fmt.Sprintf("%d", order.ExecutedAt.UnixMilli()),
			OrderType:   order.OrderType,
			Maker:       order.Maker,
			Taker:       order.Taker,
			Base:        base,
			Quote:       denom,
		}

		result = append(result, tr)
	}

	return result, nil
}
