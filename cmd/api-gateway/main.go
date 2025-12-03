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

	"github.com/nats-io/nats.go"

	"github.com/rs/cors"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	client pb.MonitoringServiceClient
	nc     *nats.Conn
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

	natsAddr := os.Getenv("NATS_ADDR")

	if natsAddr == "" {
		natsAddr = "nats://localhost:4222"
	}
	logger.Info("Connecting to NATS", "addr", natsAddr)
	nc, err := nats.Connect(natsAddr)

	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	gw := &Gateway{client: client, nc: nc}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/metrics", gw.handleGetMetrics)
	mux.HandleFunc("/metrics/demo", gw.handleDemoMetrics)
	mux.HandleFunc("/api/ingest", gw.handleIngest)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "https://datacat.com"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(mux)

	port := ":8080"
	logger.Info("API gateway starting", "port", port)

	if err := http.ListenAndServe(port, handler); err != nil {
		logger.Error("Server failed", "error", err)
	}
}

func (g *Gateway) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.Header.Get("X-API-key")
	if key != "sk_live_12345" {
		http.Error(w, "Unauthorized access...", http.StatusUnauthorized)
		return
	}

	type AgentPayload struct {
		Name      string  `json:"name"`
		Value     float64 `json:"value"`
		Timestamp int64   `json:"timestamp"`
	}

	var payload AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	pbReq := &pb.UploadRequest{
		List: []*pb.TimeSeries{
			{
				Metric: &pb.Metric{
					Name:   payload.Name,
					Labels: map[string]string{"source": "agent", "auth": "verified"},
				},
				Samples: []*pb.Sample{
					{
						Timestamp: payload.Timestamp,
						Value:     payload.Value,
					},
				},
			},
		},
	}

	bytes, err := proto.Marshal(pbReq)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if err := g.nc.Publish("metrics.upload", bytes); err != nil {
		slog.Error("Failed to publish to NATS", "error", err)
		http.Error(w, "Queue Error", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Accepted"))
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

func transformToJSON(resp *pb.GetMetricsResponse) interface{} {
	type Sample struct {
		T int64   `json:"t"`
		V float64 `json:"v"`
	}
	type Metric struct {
		Name    string   `json:"name"`
		Samples []Sample `json:"samples"`
	}
	var out []Metric
	for _, ts := range resp.List {
		m := Metric{Name: ts.Metric.Name}
		for _, s := range ts.Samples {
			m.Samples = append(m.Samples, Sample{T: s.Timestamp, V: s.Value})
		}
		out = append(out, m)
	}
	return out
}
