package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	pb "pmts/proto"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	client pb.MonitoringServiceClient
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
		logger.Error("Failed to connect to storage", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewMonitoringServiceClient(conn)

	gw := &Gateway{client: client}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/metrics", gw.handleGetMetrics)
	mux.HandleFunc("/metrics/demo", gw.handleDemoMetrics)

	port := ":8080"
	logger.Info("API gateway starting", "port", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		logger.Error("Server failed", "error", err)
	}
}

func (g *Gateway) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	req := &pb.GetMetricsRequest{MatchName: ""}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.client.GetMetrics(ctx, req)
	if err != nil {
		http.Error(w, "Failed to fetch metrics", http.StatusInternalServerError)
		slog.Error("gRPC call failed", "error", err)
		return
	}

	type JSONMetric struct {
		Name    string `json:"name"`
		Samples []struct {
			Time  int64   `json:"t"`
			Value float64 `json:"v"`
		} `json:"samples"`
	}

	var response []JSONMetric

	for _, ts := range resp.List {
		jm := JSONMetric{Name: ts.Metric.Name}
		for _, s := range ts.Samples {
			jm.Samples = append(jm.Samples, struct {
				Time  int64   `json:"t"`
				Value float64 `json:"v"`
			}{s.Timestamp, s.Value})
		}
		response = append(response, jm)
	}
	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (g *Gateway) handleDemoMetrics(w http.ResponseWriter, r *http.Request) {
	val := rand.Float64() * 100
	w.Write([]byte("# HELP cpu_usage Simulated CPU usage\n"))
	fmt.Fprintf(w, "cpu_usage_demo %f\n", val)
	w.Write([]byte("memory_usage_demo 1024\n"))
}
