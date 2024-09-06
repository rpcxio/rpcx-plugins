package client

import (
	"context"
	"fmt"
	"strings"
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
	rpcClientRequestMessageKey  = "rpc.client.request.message"
	rpcClientResponseMessageKey = "rpc.client.response."
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
	ctx0 := trace.ContextWithSpanContext(ctx.(*rc.Context).Context, spanCtx)

	spanName := fmt.Sprintf("rpcx.client.%s.%s", servicePath, serviceMethod)
	ctx1, span := p.tracer.Start(ctx0, spanName)
	span.AddEvent("PreCall", trace.WithAttributes(
		attribute.String(rpcClientRequestMessageKey, strings.ToValidUTF8(fmt.Sprintf("%+v", args), " ")),
	))
	if p.recorder != nil {
		attrs := []attribute.KeyValue{semconv.RPCService(servicePath), semconv.RPCMethod(serviceMethod)}
		p.recorder.requestsCounter.Add(ctx1, 1, metric.WithAttributes(attrs...))
		ctx.(*rc.Context).SetValue(share.OpenTelemetryStartTimeKey, time.Now())
	}
	ctx.(*rc.Context).Context = ctx1
	share.Inject(ctx, p.propagators)

	return nil
}

func (p *OpenTelemetryPlugin) PostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	span.AddEvent("PostCall", trace.WithAttributes(
		attribute.String(rpcClientResponseMessageKey, strings.ToValidUTF8(fmt.Sprintf("%+v", reply), " ")),
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
		startTime := ctx.Value(share.OpenTelemetryStartTimeKey).(time.Time)
		p.recorder.totalDuration.Record(ctx, int64(time.Since(startTime)/time.Millisecond), metric.WithAttributes(attrs...))
	}
	return nil
}
