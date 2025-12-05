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
	mux.HandleFunc("/api/register", gw.handleRegister)
	mux.HandleFunc("/api/rules", gw.handleRules)

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
	if key == "" {
		http.Error(w, "Missing API key", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	verifyResp, err := g.client.VerifyKey(ctx, &pb.VerifyKeyRequest{ApiKey: key})
	if err != nil || !verifyResp.Valid {
		slog.Warn("Invalid API Key attempt", "key", key)
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
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
		UserId: verifyResp.UserId,
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

	key := r.Header.Get("X-API-Key")
	if key == "" {
		http.Error(w, "Missing API Key", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	verifyResp, err := g.client.VerifyKey(ctx, &pb.VerifyKeyRequest{ApiKey: key})
	if err != nil || !verifyResp.Valid {
		slog.Warn("Invalid API Key attempt", "key", key)
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	req := &pb.GetMetricsRequest{
		MatchName: "",
		UserId:    verifyResp.UserId,
	}

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
	w.Write([]byte("# HELP platform_go_cpu Simulated CPU usage\n"))
	fmt.Fprintf(w, "platform_go_cpu %f\n", val)
}

func (g *Gateway) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type RegisterRequest struct {
		Email string `json:"email"`
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rpcResp, err := g.client.CreateUser(ctx, &pb.CreateUserRequest{Email: req.Email})
	if err != nil {
		http.Error(w, "RPC Failed", http.StatusInternalServerError)
		return
	}

	if rpcResp.Error != "" {
		http.Error(w, rpcResp.Error, http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"api_key": rpcResp.ApiKey,
		"user_id": fmt.Sprintf("%d", rpcResp.UserId),
	})
}
func (g *Gateway) handleRules(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		http.Error(w, "Missing Key", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	verifyResp, err := g.client.VerifyKey(ctx, &pb.VerifyKeyRequest{ApiKey: key})
	if err != nil || !verifyResp.Valid {
		http.Error(w, "Invalid Key", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodPost {
		type RulePayload struct {
			Metric    string  `json:"metric"`
			Threshold float64 `json:"threshold"`
		}
		var payload RulePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Bad JSON", http.StatusBadRequest)
			return
		}

		_, err := g.client.CreateAlertRule(ctx, &pb.CreateRuleRequest{
			UserId:     verifyResp.UserId,
			MetricName: payload.Metric,
			Threshold:  payload.Threshold,
		})
		if err != nil {
			slog.Error("Failed to create rule", "err", err)
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Rule Created"))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
