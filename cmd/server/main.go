package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"pmts/internal/alerts"
	"pmts/internal/api"
	"pmts/internal/metrics"
	"pmts/internal/scraper"

	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	logger.Info("Starting pmts...")

	store := metrics.NewMemStorage()
	ctx, cancel := context.WithCancel(context.Background())

	cfg := scraper.Config{
		Name:     "local",
		URL:      "http://localhost:8080/demo/metrics",
		Interval: 5 * time.Second,
	}

	scr := scraper.New(store, cfg)

	go scr.Start(ctx)

	alertRules := []alerts.Rule{
		{
			Name:       "CPU usage too high",
			MetricName: "app_cpu_usage",
			Treshold:   40.0,
		},
	}

	alertEngine := alerts.NewEngine(store, alertRules)
	go alertEngine.Start(ctx)

	apiServer := api.NewServer(store)
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		logger.Info("Starting API server", "addr", ":8080")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("API server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown error", "error", err)
	}
	time.Sleep(1 * time.Second)
	logger.Info("Goodbye")
}
