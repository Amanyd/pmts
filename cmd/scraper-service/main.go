package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "pmts/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ScraperConfig struct {
	Name     string
	Interval time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	addr := os.Getenv("STORAGE_ADDR")
	if addr == "" {
		addr = "localhost:5050"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	client := pb.NewMonitoringServiceClient(conn)

	cfg := ScraperConfig{
		Name:     "microservice-scraper",
		Interval: 5 * time.Second,
	}

	logger.Info("Starting scraper service", "target", "localhost:50051")

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
			scrapeAndSend(client, cfg, logger)
		}

	}

}

func scrapeAndSend(client pb.MonitoringServiceClient, cfg ScraperConfig, logger *slog.Logger) {
	// val := rand.Float64() * 100
	// logger.Info("Scrapped data", "value", val)
	targetURL := "http://api-gateway:8080/metrics/demo"
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
		List: data,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rpcResp, err := client.UploadSamples(ctx, req)
	if err != nil {
		logger.Error("Failed to upload metrics", "error", err)
		return
	}
	logger.Info("Scrape success", "stored_count", rpcResp.StoredCount)
}
