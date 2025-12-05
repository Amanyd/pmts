package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net"
	"os"
	pb "pmts/proto"
	"syscall"
	"time"

	"os/signal"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	pb.UnimplementedMonitoringServiceServer

	// mu   sync.RWMutex
	// data map[string]*pb.TimeSeries

	db *sql.DB
}

func NewServer(db *sql.DB) *Server {
	return &Server{
		db: db,
	}
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
		timestamp BIGINT NOT NULL,
		value DOUBLE PRECISION NOT NULL
	);

	CREATE TABLE IF NOT EXISTS alert_rules (
        id SERIAL PRIMARY KEY,
        user_id INTEGER REFERENCES users(id),
        metric_name TEXT NOT NULL,
        threshold DOUBLE PRECISION NOT NULL
    );

	

	CREATE INDEX IF NOT EXISTS idx_metric_name ON samples(metric_name);

	

	INSERT INTO users (email, api_key) 
	VALUES ('datacat.com', 'sk_live_12345')
	ON CONFLICT DO NOTHING;
	INSERT INTO alert_rules (user_id, metric_name, threshold)
    VALUES (1, 'platform_go_cpu', 90.0)
    ON CONFLICT DO NOTHING;
	`
	_, err := db.Exec(query)
	return err
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

func (s *Server) CreateAlertRule(ctx context.Context, req *pb.CreateRuleRequest) (*pb.CreateRuleResponse, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO alert_rules (user_id, metric_name, threshold) VALUES ($1, $2, $3) RETURNING id",
		req.UserId, req.MetricName, req.Threshold).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &pb.CreateRuleResponse{RuleId: id}, nil
}

func (s *Server) GetAlertRules(ctx context.Context, req *pb.GetRulesRequest) (*pb.GetRulesResponse, error) {
	query := "SELECT user_id, metric_name, threshold FROM alert_rules"
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
		if err := rows.Scan(&r.UserId, &r.MetricName, &r.Threshold); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return &pb.GetRulesResponse{Rules: rules}, nil
}

func generateAPIKey() string {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback_key_" + time.Now().String()
	}
	return "sk_" + hex.EncodeToString(bytes)
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
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO samples (user_id, metric_name, timestamp, value) VALUES ($1, $2, $3, $4)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, series := range list {
		name := series.Metric.Name

		for _, sample := range series.Samples {
			_, err := stmt.ExecContext(ctx, userID, name, sample.Timestamp, sample.Value)
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

func (s *Server) UploadSamples(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
	// s.mu.Lock()
	// defer s.mu.Unlock()
	uid := req.UserId
	if uid == 0 {
		uid = 1
	}
	count, err := s.persistBatch(ctx, req.List, uid)
	if err != nil {
		return nil, err
	}

	return &pb.UploadResponse{
		StoredCount: int32(count),
	}, nil
}

func (s *Server) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	// s.mu.Lock()
	// defer s.mu.Unlock()

	uid := req.UserId
	if uid == 0 {
		uid = 1
	}

	query := "SELECT metric_name, timestamp, value FROM samples WHERE user_id = $1"
	args := []interface{}{uid}

	if req.MatchName != "" {
		query += " AND metric_name = $2"
		args = append(args, req.MatchName)
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
	logger.Info("NATS listner started")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Failed to listen gRPC", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterMonitoringServiceServer(grpcServer, srv)

	logger.Info("Storage service started")

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down...")
}
