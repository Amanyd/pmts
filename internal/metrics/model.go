package metrics

type Metric struct {
	Name   string
	Labels map[string]string
}

type Sample struct {
	Timestamp int64
	Value     float64
}

type TimeSeries struct {
	Metric  Metric
	Samples []Sample
}
