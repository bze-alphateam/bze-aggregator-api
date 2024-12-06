package listener

import (
	"context"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/sirupsen/logrus"
	types2 "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/rpc/client/http"
	"strings"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"
)

const (
	tradebinStr = "tradebin"

	heartBeatInterval = time.Second * 60 * 5
)

type TradebinListener struct {
	logger logrus.FieldLogger
	client *http.HTTP
}

func NewTradebinListener(conn *http.HTTP, logger logrus.FieldLogger) (*TradebinListener, error) {
	if conn == nil || logger == nil {
		return nil, internal.NewInvalidDependenciesErr("NewTradebinListener")
	}

	return &TradebinListener{
		client: conn,
		logger: logger.WithField("service", "TradebinListener"),
	}, nil
}

func (w *TradebinListener) Listen(msgChan chan<- types2.Event) error {
	if err := w.client.Start(); err != nil {
		return fmt.Errorf("could not start ws client: %w", err)
	}

	defer w.client.Stop()

	// Subscribe to NewBlock events
	blockEventChan, err := w.client.Subscribe(context.Background(), "block-listener", "tm.event = 'NewBlock'")
	if err != nil {
		return err
	}
	defer w.client.UnsubscribeAll(context.Background(), "block-listener")

	// Subscribe to Tx events
	txEventChan, err := w.client.Subscribe(context.Background(), "tx-listener", "tm.event = 'Tx'")
	if err != nil {
		return err
	}
	defer w.client.UnsubscribeAll(context.Background(), "tx-listener")

	// Use a select statement to listen to both channels concurrently
	blockChanClosed := false
	txChanClosed := false

	// Start a ping ticker to keep the connection alive
	ticker := time.NewTicker(heartBeatInterval) // Adjust interval as needed
	defer ticker.Stop()
	w.keepAliveTicker(ticker)

	// Use a select statement to listen to both channels concurrently
	for {
		if blockChanClosed || txChanClosed {
			w.logger.Error("one of the channels was closed")
			close(msgChan)

			return nil
		}

		select {
		case blockMsg, ok := <-blockEventChan:
			if !ok {
				blockChanClosed = true
				continue
			}
			if evt, ok := blockMsg.Data.(tmtypes.EventDataNewBlock); ok {
				allEvents := append(evt.ResultBeginBlock.Events, evt.ResultEndBlock.Events...)
				for _, event := range allEvents {
					if !strings.Contains(event.Type, tradebinStr) {
						continue
					}

					msgChan <- event
				}
			}

		case txMsg, ok := <-txEventChan:
			if !ok {
				txChanClosed = true
				continue
			}
			if evt, ok := txMsg.Data.(tmtypes.EventDataTx); ok {
				txResult := evt.Result
				for _, event := range txResult.Events {
					if !strings.Contains(event.Type, tradebinStr) {
						continue
					}

					msgChan <- event
				}
			}
		}
	}
}

func (w *TradebinListener) keepAliveTicker(ticker *time.Ticker) {
	go func() {
		for {
			select {
			case <-ticker.C:
				resp, err := w.client.Health(context.Background())
				_ = resp
				if err != nil {
					w.logger.WithError(err).Error("failed to send keep alive request")
				} else {
					w.logger.Info("keep alive request success")
				}
			}
		}
	}()
}
