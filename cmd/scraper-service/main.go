package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "pmts/proto"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type ScraperConfig struct {
	Name     string
	Interval time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	natsAddr := os.Getenv("NATS_ADDR")
	if natsAddr == "" {
		natsAddr = "nats://localhost:4222"
	}
	logger.Info("Connecting to NATS", "addr", natsAddr)

	nc, err := nats.Connect(natsAddr)
	if err != nil {
		logger.Error("Failed to connect NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	cfg := ScraperConfig{
		Name:     "microservice-scraper",
		Interval: 5 * time.Second,
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-quit:
			logger.Info("Stopping scraper")
			return
		case <-ticker.C:
			scrapeAndPublish(nc, cfg, logger)
		}

	}

}

func scrapeAndPublish(nc *nats.Conn, cfg ScraperConfig, logger *slog.Logger) {
	// val := rand.Float64() * 100
	// logger.Info("Scrapped data", "value", val)
	targetURL := "http://api-gateway:8080/metrics/demo"
	if os.Getenv("InDocker") != "true" {
		targetURL = "http://localhost:8080/metrics/demo"
	}

	resp, err := http.Get(targetURL)
	if err != nil {
		logger.Error("Failed to scrape", "error", err)
		return
	}
	defer resp.Body.Close()

	data, err := parseMetrics(resp.Body)
	if err != nil {
		logger.Error("Failed to parse metrics", "error", err)
		return
	}

	req := &pb.UploadRequest{
		List:   data,
		UserId: 1,
	}

	bytes, err := proto.Marshal(req)
	if err != nil {
		logger.Info("Failed to marshal protobufs", "error", err)
		return
	}

	if err := nc.Publish("metrics.upload", bytes); err != nil {
		logger.Error("Failed to publish to NATS", "error", err)
		return
	}
	logger.Info("Published metrics to NATS", "bytes", len(bytes))
}
