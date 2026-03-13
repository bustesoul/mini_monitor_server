package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	snap := s.getSnapshot()
	var b strings.Builder

	writeMetricHeader(&b, "mini_monitor_up", "Whether mini_monitor_server has a collected snapshot available.", "gauge")
	if snap == nil {
		writeMetricValue(&b, "mini_monitor_up", nil, 0)
		_, _ = w.Write([]byte(b.String()))
		return
	}

	writeMetricValue(&b, "mini_monitor_up", nil, 1)

	writeMetricHeader(&b, "mini_monitor_snapshot_timestamp_seconds", "Unix timestamp of the latest collected snapshot.", "gauge")
	writeMetricValue(&b, "mini_monitor_snapshot_timestamp_seconds", nil, float64(snap.Timestamp.UTC().Unix()))

	writeMetricHeader(&b, "mini_monitor_cpu_usage_percent", "CPU usage percent collected by mini_monitor_server.", "gauge")
	writeMetricValue(&b, "mini_monitor_cpu_usage_percent", nil, snap.CPU.UsagePercent)

	writeMetricHeader(&b, "mini_monitor_memory_total_bytes", "Total memory in bytes.", "gauge")
	writeMetricValue(&b, "mini_monitor_memory_total_bytes", nil, float64(snap.Memory.TotalBytes))
	writeMetricHeader(&b, "mini_monitor_memory_used_bytes", "Used memory in bytes.", "gauge")
	writeMetricValue(&b, "mini_monitor_memory_used_bytes", nil, float64(snap.Memory.UsedBytes))
	writeMetricHeader(&b, "mini_monitor_memory_usage_percent", "Memory usage percent collected by mini_monitor_server.", "gauge")
	writeMetricValue(&b, "mini_monitor_memory_usage_percent", nil, snap.Memory.UsedPercent)

	writeMetricHeader(&b, "mini_monitor_storage_dir_size_bytes", "Storage directory size in bytes.", "gauge")
	writeMetricValue(&b, "mini_monitor_storage_dir_size_bytes", nil, float64(snap.StorageDirSizeBytes))

	writeMetricHeader(&b, "mini_monitor_disk_total_bytes", "Disk total size in bytes by mount.", "gauge")
	writeMetricHeader(&b, "mini_monitor_disk_used_bytes", "Disk used size in bytes by mount.", "gauge")
	writeMetricHeader(&b, "mini_monitor_disk_usage_percent", "Disk usage percent by mount.", "gauge")
	for _, disk := range snap.Disks {
		labels := map[string]string{"mount": disk.Mount}
		writeMetricValue(&b, "mini_monitor_disk_total_bytes", labels, float64(disk.TotalBytes))
		writeMetricValue(&b, "mini_monitor_disk_used_bytes", labels, float64(disk.UsedBytes))
		writeMetricValue(&b, "mini_monitor_disk_usage_percent", labels, disk.UsedPercent)
	}

	writeMetricHeader(&b, "mini_monitor_network_receive_bytes_total", "Network receive bytes since the current baseline by interface.", "counter")
	writeMetricHeader(&b, "mini_monitor_network_transmit_bytes_total", "Network transmit bytes since the current baseline by interface.", "counter")
	for _, net := range snap.Networks {
		labels := map[string]string{"iface": net.Iface}
		writeMetricValue(&b, "mini_monitor_network_receive_bytes_total", labels, float64(net.RXBytes))
		writeMetricValue(&b, "mini_monitor_network_transmit_bytes_total", labels, float64(net.TXBytes))
	}

	_, _ = w.Write([]byte(b.String()))
}

func writeMetricHeader(b *strings.Builder, name, help, typ string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s %s\n", name, typ)
}

func writeMetricValue(b *strings.Builder, name string, labels map[string]string, value float64) {
	b.WriteString(name)
	if len(labels) > 0 {
		first := true
		b.WriteByte('{')
		for key, val := range labels {
			if !first {
				b.WriteByte(',')
			}
			first = false
			fmt.Fprintf(b, `%s="%s"`, key, escapeLabelValue(val))
		}
		b.WriteByte('}')
	}
	b.WriteByte(' ')
	b.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	b.WriteByte('\n')
}

func escapeLabelValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)
	return replacer.Replace(value)
}
