package client

import (
	"context"
	"fmt"
	"time"

	rc "github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/rpcxio/rpcx-plugins/share"
)

const (
	metricRequestPath = "rpcx.client.request.path"
)

type OpenTelemetryPlugin struct {
	tracer      trace.Tracer
	recorder    *Recorder
	propagators propagation.TextMapPropagator
}

func NewOpenTelemetryPlugin(tracer trace.Tracer, propagators propagation.TextMapPropagator) *OpenTelemetryPlugin {
	if propagators == nil {
		propagators = otel.GetTextMapPropagator()
	}

	ret := &OpenTelemetryPlugin{
		tracer:      tracer,
		propagators: propagators,
	}
	return ret
}

func (p *OpenTelemetryPlugin) WithMeter(meter metric.Meter) *OpenTelemetryPlugin {
	p.recorder = GetRecorder(meter)
	return p
}

func (p *OpenTelemetryPlugin) PreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx0 := trace.ContextWithSpanContext(ctx, spanCtx)

	spanName := fmt.Sprintf("rpcx.client.%s.%s", servicePath, serviceMethod)
	ctx1, span := p.tracer.Start(ctx0, spanName)
	share.Inject(ctx1, p.propagators)
	span.AddEvent("PreCall", trace.WithAttributes(
		attribute.String("rpc.client.request.message", fmt.Sprintf("%+v", args)),
	))
	ctx.(*rc.Context).SetValue(share.OpenTelemetryKey, span)
	if p.recorder != nil {
		attrs := []attribute.KeyValue{semconv.RPCService(servicePath), semconv.RPCMethod(serviceMethod)}
		p.recorder.requestsCounter.Add(ctx1, 1, metric.WithAttributes(attrs...))
		ctx.(*rc.Context).SetValue(share.OpenTelemetryStartTimeKey, time.Now().UnixMilli())
	}

	return nil
}

func (p *OpenTelemetryPlugin) PostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	span := ctx.Value(share.OpenTelemetryKey).(trace.Span)
	defer span.End()

	span.AddEvent("PostCall", trace.WithAttributes(
		attribute.String("rpc.client.response.message", fmt.Sprintf("%+v", reply)),
	))
	attrs := []attribute.KeyValue{semconv.RPCService(servicePath), semconv.RPCMethod(serviceMethod)}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		attrs = append(attrs, semconv.OTelStatusCodeError)
	} else {
		span.SetStatus(codes.Ok, "success")
		attrs = append(attrs, semconv.OTelStatusCodeError)
	}
	if p.recorder != nil {
		p.recorder.responsesCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		startTime := ctx.Value(share.OpenTelemetryStartTimeKey).(int64)
		p.recorder.totalDuration.Record(ctx, time.Now().UnixMilli()-startTime, metric.WithAttributes(attrs...))
	}
	return nil
}
