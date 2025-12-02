package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "pmts/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	addr := os.Getenv("STORAGE_ADDR")
	if addr == "" {
		addr = "localhost:5050"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to storage", "error", err)
	}

	defer conn.Close()

	client := pb.NewMonitoringServiceClient(conn)

	logger.Info("Starting alert service")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-quit:
			logger.Info("Stopping alert service")
			return
		case <-ticker.C:
			checkAlerts(client, logger)
		}
	}
}

func checkAlerts(client pb.MonitoringServiceClient, logger *slog.Logger) {
	targetMetric := "cpu_usage_demo"
	treshold := 50.0

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &pb.GetMetricsRequest{MatchName: targetMetric}
	resp, err := client.GetMetrics(ctx, req)

	if err != nil {
		logger.Error("Failed to fetch metrics", "error", err)
		return
	}

	for _, series := range resp.List {
		if len(series.Samples) == 0 {
			continue
		}
		lastSample := series.Samples[len(series.Samples)-1]
		if lastSample.Value > treshold {
			logger.Warn(
				"ALERT FIRED",
				"metric", targetMetric,
				"value", lastSample.Value,
				"treshold", treshold,
			)
		} else {
			logger.Info("Status OK", "value", lastSample.Value)
		}
	}
}
