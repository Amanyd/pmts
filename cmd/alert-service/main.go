package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pb "pmts/proto"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type RuleCache struct {
	mu    sync.RWMutex
	rules map[int64][]*pb.AlertRule
}

func (c *RuleCache) Refresh(client pb.MonitoringServiceClient, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetAlertRules(ctx, &pb.GetRulesRequest{UserId: 0})
	if err != nil {
		logger.Error("Failed to refresh rules", "error", err)
		return
	}
	newRules := make(map[int64][]*pb.AlertRule)
	for _, r := range resp.Rules {
		newRules[r.UserId] = append(newRules[r.UserId], r)
	}

	c.mu.Lock()
	c.rules = newRules
	c.mu.Unlock()
	logger.Info("Rules refreshed", "total_count", len(resp.Rules))
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	storageAddr := os.Getenv("STORAGE_ADDR")
	if storageAddr == "" {
		storageAddr = "localhost:50051"
	}

	conn, err := grpc.NewClient(storageAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		os.Exit(1)
	}
	storageClient := pb.NewMonitoringServiceClient(conn)

	cache := &RuleCache{rules: make(map[int64][]*pb.AlertRule)}
	cache.Refresh(storageClient, logger)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			cache.Refresh(storageClient, logger)
		}
	}()

	natsAddr := os.Getenv("NATS_ADDR")
	if natsAddr == "" {
		natsAddr = "nats://localhost:4222"
	}
	nc, err := nats.Connect(natsAddr)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}

	defer nc.Close()

	logger.Info("Alert service listening on NATS", "subject", "metrics.upload")

	_, err = nc.QueueSubscribe("metrics.upload", "alert-workers", func(m *nats.Msg) {
		req := &pb.UploadRequest{}
		if err := proto.Unmarshal(m.Data, req); err != nil {
			logger.Error("Bad message", "error", err)
			return
		}
		checkRules(req.List, req.UserId, cache, logger)
	})

	if err != nil {
		logger.Error("Failed to subscribe", "error", err)
		os.Exit(1)
	}

	logger.Info("Alert Service Started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down...")
}

func checkRules(list []*pb.TimeSeries, userID int64, cache *RuleCache, logger *slog.Logger) {
	cache.mu.RLock()
	userRules, exists := cache.rules[userID]
	cache.mu.RUnlock()

	if !exists {
		return
	}

	for _, series := range list {
		for _, sample := range series.Samples {
			for _, rule := range userRules {
				if series.Metric.Name == rule.MetricName {
					if sample.Value > rule.Threshold {
						logger.Warn("ALERT",
							"user", userID,
							"metric", rule.MetricName,
							"val", sample.Value,
							"limit", rule.Threshold)
					}
				}
			}
		}
	}
}
