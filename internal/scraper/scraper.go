package scraper

import (
	"context"

	"log/slog"
	"net/http"
	"pmts/internal/metrics"
	"time"
)

type Config struct {
	Name     string
	URL      string
	Interval time.Duration
}

type Scraper struct {
	store  *metrics.MemStorage
	cfg    Config
	client *http.Client
}

func New(store *metrics.MemStorage, cfg Config) *Scraper {
	return &Scraper{
		store: store,
		cfg:   cfg,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (s *Scraper) Start(ctx context.Context) {
	logger := slog.Default().With("scraper", s.cfg.Name)
	logger.Info("Starting the scraper", "url", s.cfg.URL)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stoping scraper")
			return

		case <-ticker.C:
			s.scrape(ctx, logger)
		}

	}
}

func (s *Scraper) scrape(ctx context.Context, logger *slog.Logger) {

	req, err := http.NewRequestWithContext(ctx, "GET", s.cfg.URL, nil)
	if err != nil {
		logger.Error("Failed to create request", "error", err)
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		logger.Error("Failed to fetch metrics", "url", s.cfg.URL, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Error("Non-OK status", "status", resp.Status)
		return
	}

	data, err := parseMetrics(resp.Body)
	if err != nil {
		logger.Error("Failed to parse metrics", "error", err)
		return
	}

	for _, ts := range data {
		err := s.store.Add(ctx, ts.Metric, ts.Samples[0].Timestamp, ts.Samples[0].Value)
		if err != nil {
			logger.Error("Failed to store metric", "name", ts.Metric.Name, "error", err)
		}

	}
	logger.Info("Scrape uccesfull", "count", len(data))
}
