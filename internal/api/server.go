package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"pmts/internal/metrics"
)

type Server struct {
	store *metrics.MemStorage
}

func NewServer(store *metrics.MemStorage) *Server {
	return &Server{
		store: store,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/demo/metrics", s.handleDemoMetrics)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		slog.Error("Failed to fetch metrics", "error", err)
		return
	}
	w.Header().Set("Content-type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleDemoMetrics(w http.ResponseWriter, r *http.Request) {
	cpu := rand.Float64() * 100
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# This is a fake metrics endpoint\n"))
	fmt.Fprintf(w, "app_cpu_usage %.2f\n", cpu)
	w.Write([]byte("app_memory_usage 1024.0\n"))
	w.Write([]byte("app_requests_total 500\n"))
}
