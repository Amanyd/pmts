package metrics

import (
	"context"
	"fmt"

	"sync"
)

type MemStorage struct {
	mu sync.RWMutex

	data map[string]*TimeSeries
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		data: make(map[string]*TimeSeries),
	}
}

func (s *MemStorage) Add(ctx context.Context, m Metric, timestamp int64, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	key := m.Name
	series, exists := s.data[key]

	if !exists {
		series = &TimeSeries{
			Metric:  m,
			Samples: make([]Sample, 0),
		}
		s.data[key] = series
	}
	series.Samples = append(series.Samples, Sample{
		Timestamp: timestamp,
		Value:     value,
	})

	return nil
}
func (s *MemStorage) Get(ctx context.Context, name string) (*TimeSeries, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	series, exists := s.data[name]
	if !exists {
		return nil, fmt.Errorf("metric not found: %s", name)
	}
	return series, nil
}
func (s *MemStorage) GetAll(ctx context.Context) (map[string]*TimeSeries, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	copyMap := make(map[string]*TimeSeries)
	for k, v := range s.data {
		copyMap[k] = v
	}
	return copyMap, nil
}
