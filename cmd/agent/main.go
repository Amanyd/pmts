package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"bufio"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	gnet "github.com/shirou/gopsutil/v3/net"
)

var (
	apiKey    = flag.String("key", "", "Your API key (required)")
	ingestURL = flag.String("ingest", "http://localhost:8080/api/ingest", "Gateway ingest URL")
	scrapeURL = flag.String("scrape", "", "Optional: local URL to scrape for custom metrics")
	hostName  = flag.String("name", "", "Server name (defaults to OS hostname)")
)

type MetricPayload struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp int64             `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
}

func main() {
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("Error: --key is required")
	}
	if *hostName == "" {
		host, _ := os.Hostname()
		*hostName = host
	}

	fmt.Printf("DataCat Agent\n")
	fmt.Printf("  host:   %s\n", *hostName)
	fmt.Printf("  target: %s\n", *ingestURL)
	if *scrapeURL != "" {
		fmt.Printf("  scrape: %s\n", *scrapeURL)
	}

	for {
		var batch []MetricPayload

		batch = append(batch, collectSystemMetrics()...)

		if *scrapeURL != "" {
			batch = append(batch, collectAppMetrics()...)
		}

		if len(batch) > 0 {
			sendBatch(batch)
		}

		time.Sleep(5 * time.Second)
	}
}

func labels() map[string]string {
	return map[string]string{"host": *hostName}
}

func collectSystemMetrics() []MetricPayload {
	now := time.Now().Unix()
	var out []MetricPayload

	add := func(name string, val float64) {
		out = append(out, MetricPayload{
			Name: name, Value: val, Timestamp: now, Labels: labels(),
		})
	}

	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		add("system_cpu_percent", pct[0])
	}
	if v, err := mem.VirtualMemory(); err == nil {
		add("system_mem_percent", v.UsedPercent)
		add("system_mem_used_gb", float64(v.Used)/1e9)
		add("system_mem_total_gb", float64(v.Total)/1e9)
	}
	if d, err := disk.Usage("/"); err == nil {
		add("system_disk_percent", d.UsedPercent)
		add("system_disk_free_gb", float64(d.Free)/1e9)
	}
	if l, err := load.Avg(); err == nil {
		add("system_load_1", l.Load1)
		add("system_load_5", l.Load5)
		add("system_load_15", l.Load15)
	}
	if n, err := gnet.IOCounters(false); err == nil && len(n) > 0 {
		add("system_net_sent_bytes", float64(n[0].BytesSent))
		add("system_net_recv_bytes", float64(n[0].BytesRecv))
	}
	return out
}

func collectAppMetrics() []MetricPayload {
	resp, err := http.Get(*scrapeURL)
	if err != nil {
		log.Printf("Scrape failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	now := time.Now().Unix()
	var out []MetricPayload
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		val, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}
		out = append(out, MetricPayload{
			Name: parts[0], Value: val, Timestamp: now, Labels: labels(),
		})
	}
	return out
}

func sendBatch(batch []MetricPayload) {
	data, _ := json.Marshal(batch)
	req, _ := http.NewRequest("POST", *ingestURL, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", *apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Upload failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Printf("Server rejected batch: %s", resp.Status)
	}
}
