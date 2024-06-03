package client

import (
	"context"
	"fmt"

	rc "github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/rpcxio/rpcx-plugins/share"
)

type OpenTelemetryPlugin struct {
	tracer      trace.Tracer
	propagators propagation.TextMapPropagator
}

func NewOpenTelemetryPlugin(tracer trace.Tracer, propagators propagation.TextMapPropagator) *OpenTelemetryPlugin {
	if propagators == nil {
		propagators = otel.GetTextMapPropagator()
	}

	return &OpenTelemetryPlugin{
		tracer:      tracer,
		propagators: propagators,
	}
}

func (p *OpenTelemetryPlugin) PreCall(ctx context.Context, servicePath, serviceMethod string, args interface{}) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx0 := trace.ContextWithSpanContext(ctx, spanCtx)

	ctx1, span := p.tracer.Start(ctx0, "rpcx.client."+servicePath+"."+serviceMethod)
	share.Inject(ctx1, p.propagators)

	span.AddEvent("Call", trace.WithAttributes(
		attribute.String("rpcx.req", fmt.Sprintf("%+v", args)),
	))
	ctx.(*rc.Context).SetValue(share.OpenTelemetryKey, span)

	return nil
}

func (p *OpenTelemetryPlugin) PostCall(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, err error) error {
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
