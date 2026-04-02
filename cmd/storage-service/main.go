package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	pb "pmts/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	pb.UnimplementedMonitoringServiceServer
	db *sql.DB
}

func NewServer(db *sql.DB) *Server {
	return &Server{db: db}
}

func initDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		api_key TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS samples (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		metric_name TEXT NOT NULL,
		labels JSONB DEFAULT '{}'::jsonb,
		timestamp BIGINT NOT NULL,
		value DOUBLE PRECISION NOT NULL
	);

	CREATE TABLE IF NOT EXISTS alert_rules (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		metric_name TEXT NOT NULL,
		threshold DOUBLE PRECISION NOT NULL,
		webhook_url TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_metric_name ON samples(metric_name, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_user_metric_ts ON samples(user_id, metric_name, timestamp DESC);

	DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'alert_rules' AND column_name = 'webhook_url'
		) THEN
			ALTER TABLE alert_rules ADD COLUMN webhook_url TEXT NOT NULL DEFAULT '';
		END IF;
	END $$;
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Only seed in dev — set SEED_DATA=true explicitly
	if os.Getenv("SEED_DATA") == "true" {
		seed := `
		INSERT INTO users (email, api_key)
		VALUES ('dev@datacat.com', 'sk_live_12345')
		ON CONFLICT DO NOTHING;
		INSERT INTO alert_rules (user_id, metric_name, threshold)
		VALUES (1, 'system_cpu_percent', 90.0)
		ON CONFLICT DO NOTHING;
		`
		_, err = db.Exec(seed)
	}
	return err
}

func generateAPIKey() string {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback_key_" + time.Now().String()
	}
	return "sk_" + hex.EncodeToString(bytes)
}

func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	newKey := generateAPIKey()
	var id int64
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO users (email, api_key) VALUES ($1, $2) RETURNING id",
		req.Email, newKey).Scan(&id)
	if err != nil {
		slog.Error("Failed to create user", "error", err)
		return &pb.CreateUserResponse{Error: "Email likely already exists"}, nil
	}
	slog.Info("Created new user", "id", id, "email", req.Email)
	return &pb.CreateUserResponse{UserId: id, ApiKey: newKey}, nil
}

func (s *Server) VerifyKey(ctx context.Context, req *pb.VerifyKeyRequest) (*pb.VerifyKeyResponse, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE api_key = $1", req.ApiKey).Scan(&userID)
	if err == sql.ErrNoRows {
		return &pb.VerifyKeyResponse{Valid: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return &pb.VerifyKeyResponse{Valid: true, UserId: userID}, nil
}

func (s *Server) persistBatch(ctx context.Context, list []*pb.TimeSeries, userID int64) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO samples (user_id, metric_name, labels, timestamp, value) VALUES ($1, $2, $3, $4, $5)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, series := range list {
		name := series.Metric.Name
		labelsJSON, _ := json.Marshal(series.Metric.Labels)
		for _, sample := range series.Samples {
			_, err := stmt.ExecContext(ctx, userID, name, labelsJSON, sample.Timestamp, sample.Value)
			if err != nil {
				return 0, err
			}
			count++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Server) UploadSamples(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
	uid := req.UserId
	if uid == 0 {
		uid = 1
	}
	count, err := s.persistBatch(ctx, req.List, uid)
	if err != nil {
		return nil, err
	}
	return &pb.UploadResponse{StoredCount: int32(count)}, nil
}

func (s *Server) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	uid := req.UserId
	if uid == 0 {
		uid = 1
	}

	query := "SELECT metric_name, timestamp, value FROM samples WHERE user_id = $1"
	args := []interface{}{uid}
	argIdx := 2

	if req.MatchName != "" {
		query += " AND metric_name = $" + itoa(argIdx)
		args = append(args, req.MatchName)
		argIdx++
	}
	if req.StartTime > 0 {
		query += " AND timestamp >= $" + itoa(argIdx)
		args = append(args, req.StartTime)
		argIdx++
	}
	if req.EndTime > 0 {
		query += " AND timestamp <= $" + itoa(argIdx)
		args = append(args, req.EndTime)
		argIdx++
	}

	query += " ORDER BY timestamp ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tempMap := make(map[string]*pb.TimeSeries)
	for rows.Next() {
		var name string
		var ts int64
		var val float64
		if err := rows.Scan(&name, &ts, &val); err != nil {
			return nil, err
		}
		if _, exists := tempMap[name]; !exists {
			tempMap[name] = &pb.TimeSeries{
				Metric:  &pb.Metric{Name: name},
				Samples: []*pb.Sample{},
			}
		}
		tempMap[name].Samples = append(tempMap[name].Samples, &pb.Sample{
			Timestamp: ts,
			Value:     val,
		})
	}

	var result []*pb.TimeSeries
	for _, v := range tempMap {
		result = append(result, v)
	}
	return &pb.GetMetricsResponse{List: result}, nil
}

func (s *Server) ListMetricNames(ctx context.Context, req *pb.ListNamesRequest) (*pb.ListNamesResponse, error) {
	uid := req.UserId
	if uid == 0 {
		uid = 1
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT metric_name FROM samples WHERE user_id = $1 ORDER BY metric_name ASC", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return &pb.ListNamesResponse{Names: names}, nil
}

func (s *Server) DeleteMetric(ctx context.Context, req *pb.DeleteMetricRequest) (*pb.DeleteMetricResponse, error) {
	_, err := s.db.ExecContext(ctx, "DELETE FROM samples WHERE user_id = $1 AND metric_name = $2", req.UserId, req.MetricName)
	if err != nil {
		slog.Error("Failed to delete metric", "error", err)
		return nil, fmt.Errorf("DB error")
	}
	// Also cleanly delete any alert rules attached to this metric
	_, err = s.db.ExecContext(ctx, "DELETE FROM alert_rules WHERE user_id = $1 AND metric_name = $2", req.UserId, req.MetricName)
	if err != nil {
		slog.Error("Failed to delete alert rules for metric", "error", err)
	}

	return &pb.DeleteMetricResponse{Ok: true}, nil
}

func (s *Server) CreateAlertRule(ctx context.Context, req *pb.CreateRuleRequest) (*pb.CreateRuleResponse, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO alert_rules (user_id, metric_name, threshold, webhook_url) VALUES ($1, $2, $3, $4) RETURNING id",
		req.UserId, req.MetricName, req.Threshold, req.WebhookUrl).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &pb.CreateRuleResponse{RuleId: id}, nil
}

func (s *Server) GetAlertRules(ctx context.Context, req *pb.GetRulesRequest) (*pb.GetRulesResponse, error) {
	query := "SELECT id, user_id, metric_name, threshold, webhook_url FROM alert_rules"
	var args []interface{}

	if req.UserId != 0 {
		query += " WHERE user_id = $1"
		args = append(args, req.UserId)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*pb.AlertRule
	for rows.Next() {
		r := &pb.AlertRule{}
		if err := rows.Scan(&r.RuleId, &r.UserId, &r.MetricName, &r.Threshold, &r.WebhookUrl); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return &pb.GetRulesResponse{Rules: rules}, nil
}

func (s *Server) DeleteAlertRule(ctx context.Context, req *pb.DeleteRuleRequest) (*pb.DeleteRuleResponse, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM alert_rules WHERE id = $1 AND user_id = $2",
		req.RuleId, req.UserId)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	return &pb.DeleteRuleResponse{Ok: rows > 0}, nil
}

func startNatsListener(nc *nats.Conn, srv *Server, logger *slog.Logger) {
	nc.QueueSubscribe("metrics.upload", "storage-workers", func(m *nats.Msg) {
		req := &pb.UploadRequest{}
		if err := proto.Unmarshal(m.Data, req); err != nil {
			return
		}
		count, err := srv.persistBatch(context.Background(), req.List, req.UserId)
		if err != nil {
			logger.Error("DB Save Failed", "error", err)
			return
		}
		logger.Info("Saved batch", "count", count, "user_id", req.UserId)
	})
}

func startRetentionWorker(db *sql.DB, logger *slog.Logger) {
	retentionDays := 30
	if v := os.Getenv("RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			retentionDays = d
		}
	}

	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			cutoff := time.Now().Unix() - int64(retentionDays*86400)
			result, err := db.Exec("DELETE FROM samples WHERE timestamp < $1", cutoff)
			if err != nil {
				logger.Error("Retention cleanup failed", "error", err)
				continue
			}
			rows, _ := result.RowsAffected()
			if rows > 0 {
				logger.Info("Retention cleanup", "deleted", rows, "cutoff_days", retentionDays)
			}
		}
	}()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	connStr := os.Getenv("DB_CONN")
	if connStr == "" {
		connStr = "postgres://admin:secret@localhost:5432/pmts"
	}
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		panic(err)
	}
	if err := initDB(db); err != nil {
		panic(err)
	}

	srv := NewServer(db)

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
	startNatsListener(nc, srv, logger)
	startRetentionWorker(db, logger)
	logger.Info("NATS listener started")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Failed to listen gRPC", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMonitoringServiceServer(grpcServer, srv)

	go func() {
		logger.Info("Storage service started", "port", "50051")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down...")
	grpcServer.GracefulStop()
}
