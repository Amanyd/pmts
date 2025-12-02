package alerts

import (
	"context"
	"log/slog"
	"pmts/internal/metrics"
	"time"
)

type Rule struct {
	Name       string
	MetricName string
	Treshold   float64
}

type Engine struct {
	store *metrics.MemStorage
	rules []Rule
}

func NewEngine(store *metrics.MemStorage, rules []Rule) *Engine {
	return &Engine{
		store: store,
		rules: rules,
	}
}

func (e *Engine) Start(ctx context.Context) {
	logger := slog.Default().With("component", "alert-engine")
	logger.Info("Starting the alert engine", "rules_count", len(e.rules))

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping alert egine")
			return
		case <-ticker.C:
			e.evaluate(ctx, logger)
		}

	}
}

func (e *Engine) evaluate(ctx context.Context, logger *slog.Logger) {
	for _, rule := range e.rules {
		series, err := e.store.Get(ctx, rule.MetricName)
		if err != nil {
			continue
		}

		if len(series.Samples) == 0 {
			continue
		}

		lastSample := series.Samples[len(series.Samples)-1]

		if lastSample.Value > rule.Treshold {
			logger.Warn("ALERT FIRED",
				"rule", rule.Name,
				"currentValue", lastSample.Value,
				"treshold", rule.Treshold,
			)
		} else {
			logger.Debug("Rule passed", "rule", rule.Name)
		}
	}
}
