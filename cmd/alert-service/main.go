package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	pb "pmts/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

const alertCooldown = 5 * time.Minute

type RuleCache struct {
	mu    sync.RWMutex
	rules map[int64][]*pb.AlertRule
}

// Tracks last fire time per rule ID to prevent webhook spam
var (
	cooldownMu   sync.RWMutex
	lastFiredAt  = make(map[int64]time.Time)
)

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
	logger.Info("Rules refreshed", "total", len(resp.Rules))
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

	_, err = nc.QueueSubscribe("metrics.upload", "alert-workers", func(m *nats.Msg) {
		req := &pb.UploadRequest{}
		if err := proto.Unmarshal(m.Data, req); err != nil {
			logger.Error("Bad NATS message", "error", err)
			return
		}
		checkRules(req.List, req.UserId, cache, logger)
	})
	if err != nil {
		logger.Error("Failed to subscribe", "error", err)
		os.Exit(1)
	}

	logger.Info("Alert service started")
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
				if series.Metric.Name != rule.MetricName {
					continue
				}
				if sample.Value > rule.Threshold {
					// Dedup: skip if this rule fired within the cooldown window
					cooldownMu.RLock()
					last, seen := lastFiredAt[rule.RuleId]
					cooldownMu.RUnlock()
					if seen && time.Since(last) < alertCooldown {
						continue
					}

					logger.Warn("ALERT FIRED",
						"user", userID,
						"metric", rule.MetricName,
						"value", sample.Value,
						"threshold", rule.Threshold,
					)

					cooldownMu.Lock()
					lastFiredAt[rule.RuleId] = time.Now()
					cooldownMu.Unlock()

					if rule.WebhookUrl != "" {
						go sendWebhook(rule, sample.Value, logger)
					}
				}
			}
		}
	}
}

func sendWebhook(rule *pb.AlertRule, value float64, logger *slog.Logger) {
	payload := map[string]interface{}{
		"metric":    rule.MetricName,
		"value":     value,
		"threshold": rule.Threshold,
		"user_id":   rule.UserId,
		"fired_at":  time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rule.WebhookUrl, "application/json", bytes.NewReader(data))
	if err != nil {
		logger.Error("Webhook delivery failed", "url", rule.WebhookUrl, "error", err)
		return
	}
	defer resp.Body.Close()
	logger.Info("Webhook delivered", "url", rule.WebhookUrl, "status", resp.Status)
}
