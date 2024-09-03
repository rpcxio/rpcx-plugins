package client

import (
	"go.opentelemetry.io/otel/metric"
)

// Recorder knows how to record and measure the metrics. This
// has the required methods to be used with the HTTP
// middlewares.
type Recorder struct {
	attemptsCounter       metric.Int64UpDownCounter
	totalDuration         metric.Int64Histogram
	activeRequestsCounter metric.Int64UpDownCounter
}

func GetRecorder(meter metric.Meter, metricsPrefix string) *Recorder {
	metricName := func(metricName string) string {
		if len(metricsPrefix) > 0 {
			return metricsPrefix + "." + metricName
		}
		return metricName
	}
	attemptsCounter, _ := meter.Int64UpDownCounter(metricName("rpcx.client.request_count"), metric.WithDescription("Number of RPCX Client Requests"), metric.WithUnit("Count"))
	totalDuration, _ := meter.Int64Histogram(metricName("rpcx.client.duration"), metric.WithDescription("Time Taken by RPCX client request"), metric.WithUnit("Milliseconds"))
	activeRequestsCounter, _ := meter.Int64UpDownCounter(metricName("rpcx.client.active_requests"), metric.WithDescription("Number of RPCX client requests inflight"), metric.WithUnit("Count"))
	return &Recorder{
		attemptsCounter:       attemptsCounter,
		totalDuration:         totalDuration,
		activeRequestsCounter: activeRequestsCounter,
	}
}
