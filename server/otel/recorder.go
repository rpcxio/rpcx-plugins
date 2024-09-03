package otel

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
	requestSize           metric.Int64Histogram
	responseSize          metric.Int64Histogram
}

func GetRecorder(meter metric.Meter, metricsPrefix string) *Recorder {
	metricName := func(metricName string) string {
		if len(metricsPrefix) > 0 {
			return metricsPrefix + "." + metricName
		}
		return metricName
	}
	attemptsCounter, _ := meter.Int64UpDownCounter(metricName("rpcx.server.request_count"), metric.WithDescription("Number of RPCX Server Requests"), metric.WithUnit("Count"))
	totalDuration, _ := meter.Int64Histogram(metricName("rpcx.server.duration"), metric.WithDescription("Time Taken by RPCX server request"), metric.WithUnit("Milliseconds"))
	activeRequestsCounter, _ := meter.Int64UpDownCounter(metricName("rpcx.server.active_requests"), metric.WithDescription("Number of RPCX server requests inflight"), metric.WithUnit("Count"))
	requestSize, _ := meter.Int64Histogram(metricName("rpcx.server.request_content_length"), metric.WithDescription("RPCX server Request Size"), metric.WithUnit("Bytes"))
	responseSize, _ := meter.Int64Histogram(metricName("rpcx.server.response_content_length"), metric.WithDescription("RPCX server Response Size"), metric.WithUnit("Bytes"))
	return &Recorder{
		attemptsCounter:       attemptsCounter,
		totalDuration:         totalDuration,
		activeRequestsCounter: activeRequestsCounter,
		requestSize:           requestSize,
		responseSize:          responseSize,
	}
}
