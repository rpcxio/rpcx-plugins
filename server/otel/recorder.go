package otel

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
	requestSize      metric.Int64Histogram
	responseSize     metric.Int64Histogram
}

func GetRecorder(meter metric.Meter) *Recorder {
	requestsCounter, _ := meter.Int64UpDownCounter(semconv.RPCServerRequestsPerRPCName, metric.WithDescription(semconv.RPCServerRequestsPerRPCDescription), metric.WithUnit(semconv.RPCServerRequestsPerRPCUnit))
	responsesCounter, _ := meter.Int64UpDownCounter(semconv.RPCServerResponsesPerRPCName, metric.WithDescription(semconv.RPCServerResponsesPerRPCDescription), metric.WithUnit(semconv.RPCServerResponsesPerRPCUnit))
	totalDuration, _ := meter.Int64Histogram(semconv.RPCServerDurationName, metric.WithDescription(semconv.RPCServerDurationDescription), metric.WithUnit(semconv.RPCServerDurationUnit))
	requestSize, _ := meter.Int64Histogram(semconv.RPCServerRequestSizeName, metric.WithDescription(semconv.RPCServerRequestSizeDescription), metric.WithUnit(semconv.RPCServerRequestSizeUnit))
	responseSize, _ := meter.Int64Histogram(semconv.RPCServerResponseSizeName, metric.WithDescription(semconv.RPCServerResponseSizeDescription), metric.WithUnit(semconv.RPCServerResponseSizeUnit))
	return &Recorder{
		requestsCounter:  requestsCounter,
		responsesCounter: responsesCounter,
		totalDuration:    totalDuration,
		requestSize:      requestSize,
		responseSize:     responseSize,
	}
}
