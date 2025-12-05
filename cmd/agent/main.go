package main

import (
	"bufio"
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

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

var (
	apiKey    = flag.String("key", "", "Your API Key (Required)")
	ingestURL = flag.String("ingest", "http://localhost:8080/api/ingest", "Gateway URL")
	scrapeURL = flag.String("scrape", "", "Optional: Local URL to scrape for custom metrics")
	hostName  = flag.String("name", "", "Server Name (defaults to OS hostname)")
)

type MetricPayload struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
}

func main() {
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("Error: You must provide an API Key using --key")
	}

	if *hostName == "" {
		host, _ := os.Hostname()
		*hostName = host
	}

	fmt.Printf("DataCat Agent v2.0\n")
	fmt.Printf("   --> Server:  %s\n", *hostName)
	fmt.Printf("   --> Target:  %s\n", *ingestURL)

	for {
		collectSystemMetrics()

		if *scrapeURL != "" {
			collectAppMetrics()
		}

		time.Sleep(5 * time.Second)
	}
}

func collectSystemMetrics() {
	now := time.Now().Unix()

	percent, err := cpu.Percent(0, false)
	if err == nil && len(percent) > 0 {
		send(MetricPayload{"system_cpu_percent", percent[0], now})
	}

	v, err := mem.VirtualMemory()
	if err == nil {
		send(MetricPayload{"system_mem_percent", v.UsedPercent, now})
		send(MetricPayload{"system_mem_used_gb", float64(v.Used) / 1e9, now})
		send(MetricPayload{"system_mem_total_gb", float64(v.Total) / 1e9, now})
	}

	d, err := disk.Usage("/")
	if err == nil {
		send(MetricPayload{"system_disk_percent", d.UsedPercent, now})
		send(MetricPayload{"system_disk_free_gb", float64(d.Free) / 1e9, now})
	}

	l, err := load.Avg()
	if err == nil {
		send(MetricPayload{"system_load_1", l.Load1, now})
		send(MetricPayload{"system_load_5", l.Load5, now})
		send(MetricPayload{"system_load_15", l.Load15, now})
	}

	n, err := net.IOCounters(false)
	if err == nil && len(n) > 0 {
		send(MetricPayload{"system_net_sent_bytes", float64(n[0].BytesSent), now})
		send(MetricPayload{"system_net_recv_bytes", float64(n[0].BytesRecv), now})
	}
}

func collectAppMetrics() {
	resp, err := http.Get(*scrapeURL)
	if err != nil {
		log.Printf("Scrape failed: %v", err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	now := time.Now().Unix()

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

		send(MetricPayload{parts[0], val, now})
	}
}

func send(m MetricPayload) {
	data, _ := json.Marshal(m)
	req, _ := http.NewRequest("POST", *ingestURL, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", *apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("Upload failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		log.Printf("Server rejected: %s", resp.Status)
	}
}
