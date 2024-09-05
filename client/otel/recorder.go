package client

import (
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Recorder knows how to record and measure the metrics. This
// has the required methods to be used with the HTTP
// middlewares.
type Recorder struct {
	requestsCounter  metric.Int64UpDownCounter
	responsesCounter metric.Int64UpDownCounter
	totalDuration    metric.Int64Histogram
}

func GetRecorder(meter metric.Meter) *Recorder {
	requestsCounter, _ := meter.Int64UpDownCounter(semconv.RPCClientRequestsPerRPCName, metric.WithDescription(semconv.RPCClientRequestsPerRPCDescription), metric.WithUnit(semconv.RPCClientRequestsPerRPCUnit))
	responsesCounter, _ := meter.Int64UpDownCounter(semconv.RPCClientResponsesPerRPCName, metric.WithDescription(semconv.RPCClientResponsesPerRPCDescription), metric.WithUnit(semconv.RPCClientResponsesPerRPCUnit))
	totalDuration, _ := meter.Int64Histogram(semconv.RPCClientDurationName, metric.WithDescription(semconv.RPCClientDurationDescription), metric.WithUnit(semconv.RPCClientDurationUnit))
	return &Recorder{
		requestsCounter:  requestsCounter,
		responsesCounter: responsesCounter,
		totalDuration:    totalDuration,
	}
}
