package main

import (
	"bufio"
	"io"
	pb "pmts/proto"
	"strconv"
	"strings"
	"time"
)

func parseMetrics(body io.Reader) ([]*pb.TimeSeries, error) {
	var results []*pb.TimeSeries
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

		ts := &pb.TimeSeries{
			Metric: &pb.Metric{
				Name:   name,
				Labels: map[string]string{"source": "demo_target"},
			},
			Samples: []*pb.Sample{
				{Timestamp: now, Value: value},
			},
		}

		results = append(results, ts)
	}
	return results, scanner.Err()
}
