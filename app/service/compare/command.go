package compare

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	// DefaultAggBase is the public mainnet aggregator.
	DefaultAggBase = "https://getbze.com"
	// DefaultScanBase is BZE Scan, which serves the aggregator's paths under
	// /aggregator.
	DefaultScanBase = "https://explorer.getbze.com/aggregator"

	// RequestTimeout is the HTTP client's timeout per request.
	RequestTimeout = 30 * time.Second

	flagAgg      = "agg"
	flagScan     = "scan"
	flagInterval = "interval"
	flagDuration = "duration"
)

// ErrDiverged is returned by the command when a round diverged, so the
// process exits with code 1.
var ErrDiverged = errors.New("BZE Scan diverged from the aggregator")

// NewHTTPClient is the doer the command uses: a 30 s timeout per request.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: RequestTimeout}
}

// SystemClock is the wall clock.
type SystemClock struct{}

// Now is time.Now.
func (SystemClock) Now() time.Time {
	return time.Now()
}

// Sleep waits d or until ctx is done.
func (SystemClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// NewCommand builds `compare`. It reads no .env and needs no database or
// node: only the two public hosts.
func NewCommand(doer Doer, clock Clock, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "compare <" + strings.Join(append(Names(), "all"), "|") + ">",
		Args:      cobra.ExactArgs(1),
		ValidArgs: append(Names(), "all"),
		Short:     "Compare the aggregator's DEX endpoints with BZE Scan's copies",
		Long: `Compare the aggregator's DEX endpoints with the copies BZE Scan serves under
/aggregator, every --interval for --duration, and print the differences.

Each request is fetched from the aggregator, from Scan, then from the
aggregator again; it diverges only when Scan differs from both aggregator
answers. Exits with code 1 if any round diverged.

Usage:
./bze-agg compare tickers --duration 1m
nohup ./bze-agg compare all > compare.log 2>&1 &
`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoints, err := Lookup(args[0])
			if err != nil {
				return err
			}

			cfg := Config{}
			if cfg.AggBase, err = cmd.Flags().GetString(flagAgg); err != nil {
				return err
			}

			if cfg.ScanBase, err = cmd.Flags().GetString(flagScan); err != nil {
				return err
			}

			if cfg.Interval, err = cmd.Flags().GetDuration(flagInterval); err != nil {
				return err
			}

			if cfg.Duration, err = cmd.Flags().GetDuration(flagDuration); err != nil {
				return err
			}

			runner, err := NewRunner(doer, clock, out, cfg)
			if err != nil {
				return err
			}

			// an interrupt or kill ends the run after its summary
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			res, err := runner.Run(ctx, endpoints)
			if res.Diverged() {
				return ErrDiverged
			}

			// stopped by a signal (or the caller) before any divergence: a
			// clean run so far, exit 0
			if err != nil && ctx.Err() == nil {
				return fmt.Errorf("compare stopped: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().String(flagAgg, DefaultAggBase, "aggregator base URL")
	cmd.Flags().String(flagScan, DefaultScanBase, "BZE Scan base URL, with its /aggregator prefix")
	cmd.Flags().Duration(flagInterval, 20*time.Minute, "time between the starts of two rounds")
	cmd.Flags().Duration(flagDuration, 24*time.Hour, "how long to run; the first round starts right away")

	return cmd
}
