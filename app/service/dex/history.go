package dex

import (
	"cmp"
	"fmt"
	"slices"

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
	cgMerges    CoingeckoMerges
}

func NewHistoryService(logger logrus.FieldLogger, historyRepo historyRepo, cgMerges CoingeckoMerges) (*HistoryService, error) {
	if logger == nil || historyRepo == nil {
		return nil, internal.NewInvalidDependenciesErr("NewHistoryService")
	}

	return &HistoryService{
		logger:      logger.WithField("service", "Dex.HistoryService"),
		historyRepo: historyRepo,
		cgMerges:    cgMerges,
	}, nil
}

func (h *HistoryService) GetHistory(params *request.HistoryParams) ([]response.HistoryTrade, error) {
	hist, err := h.historyRepo.GetHistoryBy(withMarketId(params))
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

// GetCoingeckoHistory returns a market's trades split by side. For a market
// other markets are folded into (see CoingeckoMerges), the trades of all of
// them, newest first and cut to the limit. Trade ids are the history row ids,
// unique across markets.
func (h *HistoryService) GetCoingeckoHistory(params *request.HistoryParams) (*response.CoingeckoHistory, error) {
	p := withMarketId(params)
	hist, err := h.historyRepo.GetHistoryBy(p)
	if err != nil {
		return nil, err
	}

	folded := h.cgMerges.foldedInto(p.MarketId)
	for _, marketId := range folded {
		fp := p
		fp.MarketId = marketId
		fHist, err := h.historyRepo.GetHistoryBy(fp)
		if err != nil {
			return nil, err
		}

		hist = append(hist, fHist...)
	}

	if len(folded) > 0 {
		slices.SortStableFunc(hist, func(a, b entity.MarketHistory) int {
			if c := b.ExecutedAt.Compare(a.ExecutedAt); c != 0 {
				return c
			}

			return cmp.Compare(b.ID, a.ID)
		})

		if p.Limit > 0 && len(hist) > p.Limit {
			hist = hist[:p.Limit]
		}
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

// withMarketId returns a copy of params whose market id is resolved from the
// ticker id when only that was given, so a ticker_id request is filtered by
// its market instead of returning every market's trades.
func withMarketId(params *request.HistoryParams) request.HistoryParams {
	p := *params
	p.MarketId = params.MustGetMarketId()

	return p
}
