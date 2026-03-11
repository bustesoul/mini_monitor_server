package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"mini_monitor_server/internal/httpapi"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/rule"
	"mini_monitor_server/internal/storage"
)

func TestHealthz(t *testing.T) {
	srv := newTestServer(t, nil)
	resp := doRequest(t, srv, "/healthz")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestReportNoData(t *testing.T) {
	srv := newTestServer(t, nil)
	resp := doRequest(t, srv, "/report")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestReportText(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "testhost",
		CPU:       model.CPUStat{UsagePercent: 25.0},
		Memory:    model.MemoryStat{UsedPercent: 50.0},
	}
	srv := newTestServer(t, snap)
	resp := doRequest(t, srv, "/report")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if len(body) == 0 {
		t.Error("empty response body")
	}
}

func TestReportJSON(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "testhost",
		CPU:       model.CPUStat{UsagePercent: 25.0},
	}
	srv := newTestServer(t, snap)
	resp := doRequest(t, srv, "/report?format=json")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestDiskHistory(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	store.AppendDiskHistory(model.DiskStat{Mount: "/", UsedPercent: 40}, time.Now())

	snap := &model.Snapshot{Timestamp: time.Now(), Hostname: "h"}
	srv := newTestServerWithStore(t, snap, store)
	resp := doRequest(t, srv, "/history/disk?days=7")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var entries []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
}

func TestAlerts(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	store.AppendAlert(&model.AlertEvent{
		Timestamp: time.Now(), Rule: "r1", Status: "firing", Value: 91, Severity: "critical",
	})

	snap := &model.Snapshot{Timestamp: time.Now(), Hostname: "h"}
	srv := newTestServerWithStore(t, snap, store)
	resp := doRequest(t, srv, "/alerts?limit=10")
	defer resp.Body.Close()

	var alerts []model.AlertEvent
	json.NewDecoder(resp.Body).Decode(&alerts)
	if len(alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(alerts))
	}
}

func TestReportAvgQueryPassThrough(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "testhost",
		CPU:       model.CPUStat{UsagePercent: 25.0},
	}
	var got []int
	srv := newTestServerWithAvg(t, snap, func(windows []int) model.MetricsAvg {
		got = append([]int(nil), windows...)
		return model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	})
	resp := doRequest(t, srv, "/report?avg=10080")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !reflect.DeepEqual(got, []int{10080}) {
		t.Fatalf("avg windows = %v, want [10080]", got)
	}
}

// --- helpers ---

func newTestServer(t *testing.T, snap *model.Snapshot) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	store, _ := storage.New(dir)
	return newTestServerWithStoreAndAvg(t, snap, store, nil)
}

func newTestServerWithStore(t *testing.T, snap *model.Snapshot, store *storage.Storage) *httptest.Server {
	t.Helper()
	return newTestServerWithStoreAndAvg(t, snap, store, nil)
}

func newTestServerWithAvg(t *testing.T, snap *model.Snapshot, getAvg func([]int) model.MetricsAvg) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	store, _ := storage.New(dir)
	return newTestServerWithStoreAndAvg(t, snap, store, getAvg)
}

func newTestServerWithStoreAndAvg(t *testing.T, snap *model.Snapshot, store *storage.Storage, getAvg func([]int) model.MetricsAvg) *httptest.Server {
	t.Helper()
	engine := rule.NewEngine(nil)
	getSnap := func() *model.Snapshot { return snap }
	srv := httpapi.NewServer("127.0.0.1:0", getSnap, getAvg, engine, store, 7)
	return httptest.NewServer(srv.Handler())
}

func doRequest(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s error: %v", path, err)
	}
	return resp
}
