package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"mini_monitor_server/internal/metrics"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/report"
	"mini_monitor_server/internal/rule"
	"mini_monitor_server/internal/storage"
)

type Server struct {
	addr          string
	srv           *http.Server
	mux           *http.ServeMux
	getSnapshot   func() *model.Snapshot
	getMetricsAvg func(time.Time, []int) model.MetricsAvg
	engine        *rule.Engine
	store         *storage.Storage
	historyDays   int
}

func NewServer(addr string, getSnapshot func() *model.Snapshot, getMetricsAvg func(time.Time, []int) model.MetricsAvg, engine *rule.Engine, store *storage.Storage, historyDays int) *Server {
	s := &Server{
		addr:          addr,
		getSnapshot:   getSnapshot,
		getMetricsAvg: getMetricsAvg,
		engine:        engine,
		store:         store,
		historyDays:   historyDays,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/report", s.handleReport)
	mux.HandleFunc("/history/disk", s.handleDiskHistory)
	mux.HandleFunc("/history/net", s.handleNetHistory)
	mux.HandleFunc("/alerts", s.handleAlerts)

	s.mux = mux
	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

// Handler 返回 HTTP handler（供测试使用）
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	slog.Info("http server starting", "addr", s.addr)
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	snap := s.getSnapshot()
	if snap == nil {
		http.Error(w, "no data yet", http.StatusServiceUnavailable)
		return
	}

	windows := s.parseAvgWindows(r)
	var avg model.MetricsAvg
	if s.getMetricsAvg != nil {
		avg = s.getMetricsAvg(snap.Timestamp, windows)
	} else {
		avg = model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	}

	firing := s.engine.FiringRules()

	if r.URL.Query().Get("format") == "json" {
		data, err := report.JSONReport(snap, firing, windows, avg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(report.TextReport(snap, firing, windows, avg)))
}

func (s *Server) parseAvgWindows(r *http.Request) []int {
	return metrics.ParseWindows(r.URL.Query().Get("avg"), metrics.DefaultAvgWindows)
}

func (s *Server) handleDiskHistory(w http.ResponseWriter, r *http.Request) {
	days := parseQueryInt(r, "days", s.historyDays)
	entries, err := s.store.ReadDiskHistory(days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, entries)
}

func (s *Server) handleNetHistory(w http.ResponseWriter, r *http.Request) {
	days := parseQueryInt(r, "days", s.historyDays)
	entries, err := s.store.ReadNetHistory(days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, entries)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	limit := parseQueryInt(r, "limit", 20)
	alerts, err := s.store.ReadAlerts(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, alerts)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func parseQueryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
