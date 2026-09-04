package backfill

import (
	"fmt"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
	"github.com/bze-alphateam/bze-aggregator-api/app/service/converter"
	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

// swapEventType is the only event this command cares about. LP swaps are not
// part of chain state, so replaying these events is the only way to recover
// them.
const swapEventType = "bze.tradebin.SwapEvent"

// blockEventTxIndex marks a swap that was not emitted by a transaction. Swaps
// also happen in block hooks - the hourly tx-fee conversion and the burner swap
// collected fees to the native denom - and those events belong to the block,
// not to a tx. Sorting them before the block's transactions matches the order
// the Postgres indexer stores them in.
const blockEventTxIndex = -1

// rawSwapEvent is a swap event located inside a block, before it is parsed.
type rawSwapEvent struct {
	Height     int64
	TxIndex    int
	EventIndex int
	Attributes []entity.EventAttribute
}

// extractSwapEvents pulls the swap events out of a block's ABCI results, both
// the ones emitted by transactions and the ones emitted by block hooks - the
// live sync ingests both, so the back-fill has to as well.
//
// Transactions that failed (code != 0) are skipped: they still emit their
// events but never changed state, so they must not produce history rows. Block
// events have no such flag - a failing hook does not produce them.
func extractSwapEvents(height int64, res *coretypes.ResultBlockResults) []rawSwapEvent {
	if res == nil {
		return nil
	}

	var found []rawSwapEvent
	for eventIndex, event := range res.FinalizeBlockEvents {
		if event.Type != swapEventType {
			continue
		}

		found = append(found, rawSwapEvent{
			Height:     height,
			TxIndex:    blockEventTxIndex,
			EventIndex: eventIndex,
			Attributes: toEventAttributes(event.Attributes),
		})
	}

	for txIndex, tx := range res.TxsResults {
		if tx == nil || tx.Code != 0 {
			continue
		}

		for eventIndex, event := range tx.Events {
			if event.Type != swapEventType {
				continue
			}

			found = append(found, rawSwapEvent{
				Height:     height,
				TxIndex:    txIndex,
				EventIndex: eventIndex,
				Attributes: toEventAttributes(event.Attributes),
			})
		}
	}

	return found
}

// toEventAttributes adapts ABCI attributes to the shape the existing swap
// converter expects. The values are identical to what the Postgres indexer
// stores (protobuf-JSON: quoted strings and JSON coins) - only the transport
// differs - so the whole parsing path is shared with the live sync.
func toEventAttributes(attrs []abci.EventAttribute) []entity.EventAttribute {
	result := make([]entity.EventAttribute, 0, len(attrs))
	for _, attr := range attrs {
		result = append(result, entity.EventAttribute{
			Key:   attr.Key,
			Value: attr.Value,
		})
	}

	return result
}

// toStagedSwap parses a raw event through the live conversion path and turns it
// into a row ready for the staging table.
func toStagedSwap(assets assetProvider, raw rawSwapEvent, executedAt time.Time) (*entity.StagedSwap, error) {
	event := &entity.Event{
		BlockHeight: raw.Height,
		Type:        swapEventType,
		CreatedAt:   executedAt,
	}

	swapData, err := converter.ConvertEventToSwapData(event, raw.Attributes)
	if err != nil {
		return nil, fmt.Errorf("error parsing swap event data: %w", err)
	}

	conv, err := converter.NewTypesConverter(assets, swapData.GetBase().Denom, swapData.GetQuote().Denom)
	if err != nil {
		return nil, fmt.Errorf("error creating types converter: %w", err)
	}

	hist, err := conv.SwapDataToHistoryEntity(*swapData)
	if err != nil {
		return nil, fmt.Errorf("error creating market history entry: %w", err)
	}

	// the block header time is the trade time - same as the live sync does
	hist.ExecutedAt = executedAt

	return &entity.StagedSwap{
		MarketID:    hist.MarketID,
		OrderType:   hist.OrderType,
		Amount:      hist.Amount,
		Price:       hist.Price,
		ExecutedAt:  hist.ExecutedAt,
		Maker:       hist.Maker,
		Taker:       hist.Taker,
		QuoteAmount: hist.QuoteAmount,
		Height:      raw.Height,
		TxIndex:     raw.TxIndex,
		EventIndex:  raw.EventIndex,
	}, nil
}
