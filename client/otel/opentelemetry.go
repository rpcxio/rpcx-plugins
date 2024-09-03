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
	"go.opentelemetry.io/otel/trace"

	"github.com/bububa/rpcx-plugins/share"
)

const (
	metricRequestPath = "rpcx.client.request.path"
)

type OpenTelemetryPlugin struct {
	tracer      trace.Tracer
	recorder    *Recorder
	propagators propagation.TextMapPropagator
}

func NewOpenTelemetryPlugin(tracer trace.Tracer, propagators propagation.TextMapPropagator, meter metric.Meter) *OpenTelemetryPlugin {
	if propagators == nil {
		propagators = otel.GetTextMapPropagator()
	}

	ret := &OpenTelemetryPlugin{
		tracer:      tracer,
		propagators: propagators,
	}
	if meter != nil {
		ret.recorder = GetRecorder(meter, "")
	}
	return ret
}

func (p *OpenTelemetryPlugin) PreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx0 := trace.ContextWithSpanContext(ctx, spanCtx)

	spanName := fmt.Sprintf("rpcx.client.%s.%s", servicePath, serviceMethod)
	ctx1, span := p.tracer.Start(ctx0, spanName)
	share.Inject(ctx1, p.propagators)
	span.AddEvent("Call", trace.WithAttributes(
		attribute.String("rpcx.req", fmt.Sprintf("%+v", args)),
	))
	ctx.(*rc.Context).SetValue(share.OpenTelemetryKey, span)
	if p.recorder != nil {
		p.recorder.activeRequestsCounter.Add(ctx1, 1, metric.WithAttributes(attribute.String(metricRequestPath, spanName)))
		ctx.(*rc.Context).SetValue(share.OpenTelemetryStartTimeKey, time.Now().UnixMilli())
	}

	return nil
}

func (p *OpenTelemetryPlugin) PostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
	spanName := fmt.Sprintf("rpcx.client.%s.%s", servicePath, serviceMethod)
	attrs := metric.WithAttributes(attribute.String(metricRequestPath, spanName))
	if p.recorder != nil {
		p.recorder.activeRequestsCounter.Add(ctx, -1, attrs)
		p.recorder.attemptsCounter.Add(ctx, 1, attrs)
		startTime := ctx.Value(share.OpenTelemetryStartTimeKey).(int64)
		p.recorder.totalDuration.Record(ctx, time.Now().UnixMilli()-startTime, attrs)
	}
	span := ctx.Value(share.OpenTelemetryKey).(trace.Span)
	defer span.End()

	span.SetAttributes(
		attribute.String("rpcx.resp", fmt.Sprintf("%+v", reply)),
	)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}
