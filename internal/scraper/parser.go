package scraper

import (
	"bufio"
	"io"
	"pmts/internal/metrics"
	"strconv"
	"strings"
	"time"
)

func parseMetrics(body io.Reader) ([]metrics.TimeSeries, error) {
	var results []metrics.TimeSeries
	scanner := bufio.NewScanner(body)
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

		name := parts[0]
		valueStr := parts[1]

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		results = append(results, metrics.TimeSeries{
			Metric: metrics.Metric{
				Name:   name,
				Labels: map[string]string{"source": "demo"},
			},
			Samples: []metrics.Sample{
				{Timestamp: now, Value: value},
			},
		})
	}
	return results, scanner.Err()
}
