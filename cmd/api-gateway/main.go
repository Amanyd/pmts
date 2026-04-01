package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	pb "pmts/proto"
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
	nc, err := nats.Connect(natsAddr)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	gw := &Gateway{client: client, nc: nc}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", gw.handleHealth)
	mux.HandleFunc("/api/metrics", gw.handleGetMetrics)
	mux.HandleFunc("/api/metrics/names", gw.handleMetricNames)
	mux.HandleFunc("/api/ingest", gw.handleIngest)
	mux.HandleFunc("/api/register", gw.handleRegister)
	mux.HandleFunc("/api/rules", gw.handleRules)
	mux.HandleFunc("/metrics/demo", gw.handleDemoMetrics)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "https://datacat.com"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: c.Handler(mux),
	}

	go func() {
		logger.Info("API gateway starting", "port", "8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleDemoMetrics(w http.ResponseWriter, r *http.Request) {
	val := rand.Float64() * 100
	w.Write([]byte("# HELP platform_go_cpu Simulated CPU usage\n"))
	fmt.Fprintf(w, "platform_go_cpu %f\n", val)
}

func (g *Gateway) verifyKey(r *http.Request, w http.ResponseWriter) (int64, bool) {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		http.Error(w, "Missing API key", http.StatusUnauthorized)
		return 0, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	resp, err := g.client.VerifyKey(ctx, &pb.VerifyKeyRequest{ApiKey: key})
	if err != nil || !resp.Valid {
		slog.Warn("Invalid API key attempt")
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return 0, false
	}
	return resp.UserId, true
}

func (g *Gateway) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := g.verifyKey(r, w)
	if !ok {
		return
	}

	type AgentPayload struct {
		Name      string            `json:"name"`
		Value     float64           `json:"value"`
		Timestamp int64             `json:"timestamp"`
		Labels    map[string]string `json:"labels"`
	}

	buildTS := func(p AgentPayload) *pb.TimeSeries {
		ts := p.Timestamp
		if ts == 0 {
			ts = time.Now().Unix()
		}
		return &pb.TimeSeries{
			Metric:  &pb.Metric{Name: p.Name, Labels: p.Labels},
			Samples: []*pb.Sample{{Timestamp: ts, Value: p.Value}},
		}
	}

	var payloads []AgentPayload
	// Try array first, then single object
	if err := json.NewDecoder(r.Body).Decode(&payloads); err != nil || len(payloads) == 0 {
		// Try as single
		r.Body.Close()
		http.Error(w, "Bad JSON: expected object or array", http.StatusBadRequest)
		return
	}

	var list []*pb.TimeSeries
	for _, p := range payloads {
		list = append(list, buildTS(p))
	}

	pbReq := &pb.UploadRequest{UserId: userID, List: list}
	data, err := proto.Marshal(pbReq)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if err := g.nc.Publish("metrics.upload", data); err != nil {
		slog.Error("Failed to publish to NATS", "error", err)
		http.Error(w, "Queue error", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Accepted"))
}

func (g *Gateway) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		g.handleDeleteMetric(w, r)
		return
	}
	userID, ok := g.verifyKey(r, w)
	if !ok {
		return
	}

	q := r.URL.Query()
	req := &pb.GetMetricsRequest{
		UserId:    userID,
		MatchName: q.Get("name"),
	}
	if v := q.Get("from"); v != "" {
		req.StartTime, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := q.Get("to"); v != "" {
		req.EndTime, _ = strconv.ParseInt(v, 10, 64)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := g.client.GetMetrics(ctx, req)
	if err != nil {
		slog.Error("GetMetrics gRPC failed", "error", err)
		http.Error(w, "Failed to fetch metrics", http.StatusInternalServerError)
		return
	}

	type JSONSample struct {
		T int64   `json:"t"`
		V float64 `json:"v"`
	}
	type JSONMetric struct {
		Name    string       `json:"name"`
		Samples []JSONSample `json:"samples"`
	}

	var result []JSONMetric
	for _, ts := range resp.List {
		jm := JSONMetric{Name: ts.Metric.Name}
		for _, s := range ts.Samples {
			jm.Samples = append(jm.Samples, JSONSample{T: s.Timestamp, V: s.Value})
		}
		result = append(result, jm)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleDeleteMetric(w http.ResponseWriter, r *http.Request) {
	userID, ok := g.verifyKey(r, w)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing metric name", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err := g.client.DeleteMetric(ctx, &pb.DeleteMetricRequest{
		UserId:     userID,
		MetricName: name,
	})
	if err != nil {
		slog.Error("gRPC DeleteMetric failed", "error", err)
		http.Error(w, "Failed to delete metric", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (g *Gateway) handleMetricNames(w http.ResponseWriter, r *http.Request) {
	userID, ok := g.verifyKey(r, w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := g.client.ListMetricNames(ctx, &pb.ListNamesRequest{UserId: userID})
	if err != nil {
		slog.Error("ListMetricNames gRPC failed", "error", err)
		http.Error(w, "Failed to list names", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Names)
}

func (g *Gateway) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rpcResp, err := g.client.CreateUser(ctx, &pb.CreateUserRequest{Email: req.Email})
	if err != nil {
		http.Error(w, "RPC failed", http.StatusInternalServerError)
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
	userID, ok := g.verifyKey(r, w)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		resp, err := g.client.GetAlertRules(ctx, &pb.GetRulesRequest{UserId: userID})
		if err != nil {
			slog.Error("GetAlertRules gRPC failed", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		type RuleJSON struct {
			ID         int64   `json:"id"`
			MetricName string  `json:"metric"`
			Threshold  float64 `json:"threshold"`
			WebhookURL string  `json:"webhook_url"`
		}
		var rules []RuleJSON
		for _, r := range resp.Rules {
			rules = append(rules, RuleJSON{
				ID:         r.RuleId,
				MetricName: r.MetricName,
				Threshold:  r.Threshold,
				WebhookURL: r.WebhookUrl,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)

	case http.MethodPost:
		var payload struct {
			Metric     string  `json:"metric"`
			Threshold  float64 `json:"threshold"`
			WebhookURL string  `json:"webhook_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Bad JSON", http.StatusBadRequest)
			return
		}
		_, err := g.client.CreateAlertRule(ctx, &pb.CreateRuleRequest{
			UserId:     userID,
			MetricName: payload.Metric,
			Threshold:  payload.Threshold,
			WebhookUrl: payload.WebhookURL,
		})
		if err != nil {
			slog.Error("CreateAlertRule gRPC failed", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Rule created"))

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "Missing rule id", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}
		resp, err := g.client.DeleteAlertRule(ctx, &pb.DeleteRuleRequest{
			RuleId: id,
			UserId: userID,
		})
		if err != nil || !resp.Ok {
			http.Error(w, "Delete failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Deleted"))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
