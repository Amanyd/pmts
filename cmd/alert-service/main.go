package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	pb "pmts/proto"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
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
		checkRules(req.List, logger)
	})

	if err != nil {
		logger.Error("Failed to subscribe", "error", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down...")
}

func checkRules(list []*pb.TimeSeries, logger *slog.Logger) {
	targetMetric := "cpu_usage_demo"
	treshold := 50.0

	for _, series := range list {
		if series.Metric.Name != targetMetric {
			continue
		}
		for _, sample := range series.Samples {
			if sample.Value > treshold {
				logger.Warn(
					"ALERT FIRED",
					"metric", targetMetric,
					"value", sample.Value,
					"treshold", treshold,
				)
			} else {
				logger.Info("Status OK", "value", sample.Value)
			}
		}
	}
}
