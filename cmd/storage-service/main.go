package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"os"
	pb "pmts/proto"

	_ "github.com/jackc/pgx/v5/stdlib"

	"google.golang.org/grpc"
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
	CREATE TABLE IF NOT EXISTS samples (
		id SERIAL PRIMARY KEY,
		metric_name TEXT NOT NULL,
		timestamp BIGINT NOT NULL,
		value DOUBLE PRECISION NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_metric_name ON samples(metric_name);
	`
	_, err := db.Exec(query)
	return err
}

func (s *Server) UploadSamples(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
	// s.mu.Lock()
	// defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	count := 0

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO samples (metric_name, timestamp, value) VALUES ($1, $2, $3)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for _, series := range req.List {
		name := series.Metric.Name

		for _, sample := range series.Samples {
			_, err := stmt.ExecContext(ctx, name, sample.Timestamp, sample.Value)
			if err != nil {
				return nil, err
			}
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	slog.Info("Persisted samples", "count", count)

	return &pb.UploadResponse{
		StoredCount: int32(count),
	}, nil
}

func (s *Server) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	// s.mu.Lock()
	// defer s.mu.Unlock()

	query := "SELECT metric_name, timestamp, value FROM samples"
	args := []interface{}{}

	if req.MatchName != "" {
		query += " WHERE metric_name = $1"
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
		logger.Error("Failed to open DB", "error", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Error("Failed to connect to DB", "error", err)
		os.Exit(1)
	}
	logger.Info("Connected to PostgreSQL")

	if err := initDB(db); err != nil {
		logger.Error("Failed to migrate DB", "error", err)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterMonitoringServiceServer(grpcServer, NewServer(db))

	logger.Info("Storage service SQL started on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("Failed to serve", "error", err)
	}
}
