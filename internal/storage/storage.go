package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mini_monitor_server/internal/model"
)

const (
	stateFile      = "state.json"
	diskHistFile   = "disk_history.ndjson"
	netHistFile    = "net_history.ndjson"
	alertsFile     = "alerts.ndjson"
)

type Storage struct {
	dir string
	mu  sync.Mutex
}

func New(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &Storage{dir: dir}, nil
}

// --- state.json ---

func (s *Storage) LoadState() (*model.ServiceState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state model.ServiceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state.json: %w", err)
	}
	return &state, nil
}

func (s *Storage) SaveState(state *model.ServiceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, stateFile)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

// --- NDJSON append/read ---

func (s *Storage) AppendDiskHistory(entry model.DiskStat, ts time.Time) error {
	rec := struct {
		Timestamp   time.Time `json:"ts"`
		Mount       string    `json:"mount"`
		UsedPercent float64   `json:"used_percent"`
		UsedBytes   uint64    `json:"used_bytes"`
		TotalBytes  uint64    `json:"total_bytes"`
	}{ts, entry.Mount, entry.UsedPercent, entry.UsedBytes, entry.TotalBytes}
	return s.appendNDJSON(diskHistFile, rec)
}

func (s *Storage) AppendNetHistory(entry model.NetworkStat, ts time.Time) error {
	rec := struct {
		Timestamp time.Time `json:"ts"`
		Iface     string    `json:"iface"`
		RXBytes   uint64    `json:"rx_bytes"`
		TXBytes   uint64    `json:"tx_bytes"`
	}{ts, entry.Iface, entry.RXBytes, entry.TXBytes}
	return s.appendNDJSON(netHistFile, rec)
}

func (s *Storage) AppendAlert(evt *model.AlertEvent) error {
	return s.appendNDJSON(alertsFile, evt)
}

func (s *Storage) appendNDJSON(filename string, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, filename)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// --- NDJSON read ---

type DiskHistoryEntry struct {
	Timestamp   time.Time `json:"ts"`
	Mount       string    `json:"mount"`
	UsedPercent float64   `json:"used_percent"`
	UsedBytes   uint64    `json:"used_bytes"`
	TotalBytes  uint64    `json:"total_bytes"`
}

type NetHistoryEntry struct {
	Timestamp time.Time `json:"ts"`
	Iface     string    `json:"iface"`
	RXBytes   uint64    `json:"rx_bytes"`
	TXBytes   uint64    `json:"tx_bytes"`
}

func (s *Storage) ReadDiskHistory(days int) ([]DiskHistoryEntry, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	var entries []DiskHistoryEntry
	err := s.readNDJSON(diskHistFile, func(line []byte) error {
		var e DiskHistoryEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil // skip malformed
		}
		if e.Timestamp.After(cutoff) {
			entries = append(entries, e)
		}
		return nil
	})
	return entries, err
}

func (s *Storage) ReadNetHistory(days int) ([]NetHistoryEntry, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	var entries []NetHistoryEntry
	err := s.readNDJSON(netHistFile, func(line []byte) error {
		var e NetHistoryEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil
		}
		if e.Timestamp.After(cutoff) {
			entries = append(entries, e)
		}
		return nil
	})
	return entries, err
}

func (s *Storage) ReadAlerts(limit int) ([]model.AlertEvent, error) {
	var all []model.AlertEvent
	err := s.readNDJSON(alertsFile, func(line []byte) error {
		var e model.AlertEvent
		if err := json.Unmarshal(line, &e); err != nil {
			return nil
		}
		all = append(all, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 返回最后 limit 条
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}

func (s *Storage) readNDJSON(filename string, fn func([]byte) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, filename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := fn(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// CleanHistory 清理超过 keepDays 天的历史记录
func (s *Storage) CleanHistory(keepDays int) {
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, file := range []string{diskHistFile, netHistFile, alertsFile} {
		if err := s.cleanNDJSON(file, cutoff); err != nil {
			slog.Warn("clean history failed", "file", file, "error", err)
		}
	}
}

func (s *Storage) cleanNDJSON(filename string, cutoff time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, filename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var kept [][]byte
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var ts struct {
			Timestamp time.Time `json:"ts"`
		}
		if json.Unmarshal(line, &ts) == nil && ts.Timestamp.Before(cutoff) {
			continue
		}
		kept = append(kept, append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// 重写文件
	tmp := path + ".tmp"
	wf, err := os.Create(tmp)
	if err != nil {
		return err
	}
	for _, line := range kept {
		wf.Write(append(line, '\n'))
	}
	if err := wf.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// atomicWrite 原子写文件
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
