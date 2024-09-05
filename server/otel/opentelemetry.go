package otel

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"
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

var (
	_ server.RegisterPlugin          = (*OpenTelemetryPlugin)(nil)
	_ server.PostConnAcceptPlugin    = (*OpenTelemetryPlugin)(nil)
	_ server.PreHandleRequestPlugin  = (*OpenTelemetryPlugin)(nil)
	_ server.PostWriteResponsePlugin = (*OpenTelemetryPlugin)(nil)
)

type OpenTelemetryPlugin struct {
	tracer      trace.Tracer
	recorder    *Recorder
	propagators propagation.TextMapPropagator
}

const (
	tracingEventRpcxPreHandleRequest          = "rpcx.pre.handle.request"
	tracingEventRpcxPreHandleRequestPath      = "rpcx.pre.handle.request.path"
	tracingEventRpcxPreHandleRequestMetadata  = "rpcx.pre.handle.request.metadata"
	tracingEventRpcxPreHandleRequestPayload   = "rpcx.pre.handle.request.payload"
	tracingEventRpcxPostWriteResponse         = "rpcx.post.write.response"
	tracingEventRpcxPostWriteResponseMetadata = "rpcx.post.write.response.metadata"
	tracingEventRpcxPostWriteResponsePayload  = "rpcx.post.write.response.payload"
)

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

func (p OpenTelemetryPlugin) Register(name string, rcvr interface{}, metadata string) error {
	_, span := p.tracer.Start(context.Background(), "rpcx.Register")
	defer span.End()

	span.SetAttributes(attribute.String("register_service", name))

	return nil
}

func (p OpenTelemetryPlugin) Unregister(name string) error {
	_, span := p.tracer.Start(context.Background(), "rpcx.Unregister")
	defer span.End()

	span.SetAttributes(attribute.String("register_service", name))

	return nil
}

func (p OpenTelemetryPlugin) RegisterFunction(serviceName, fname string, fn interface{}, metadata string) error {
	_, span := p.tracer.Start(context.Background(), "rpcx.RegisterFunction")
	defer span.End()

	span.SetAttributes(attribute.String("register_function", serviceName+"."+fname))

	return nil
}

func (p OpenTelemetryPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	_, span := p.tracer.Start(context.Background(), "rpcx.AcceptConn")
	defer span.End()

	span.SetAttributes(attribute.String("remote_addr", conn.RemoteAddr().String()))

	return conn, true
}

func (p OpenTelemetryPlugin) PreHandleRequest(ctx context.Context, r *protocol.Message) error {
	spanCtx := share.Extract(ctx, p.propagators)
	ctx0 := trace.ContextWithRemoteSpanContext(ctx, spanCtx)

	spanName := fmt.Sprintf("rpcx.service.%s.%s", r.ServicePath, r.ServiceMethod)
	ctx1, span := p.tracer.Start(
		ctx0,
		spanName,
		trace.WithSpanKind(trace.SpanKindServer),
	)
	share.Inject(ctx1, p.propagators)
	span.AddEvent(tracingEventRpcxPreHandleRequest, trace.WithAttributes(
		attribute.String(tracingEventRpcxPreHandleRequestPath, spanName),
		attribute.String(tracingEventRpcxPreHandleRequestMetadata, fmt.Sprintf("%+v", r.Metadata)),
		attribute.String(tracingEventRpcxPreHandleRequestPayload, string(r.Payload)),
	))
	ctx.(*rc.Context).SetValue(share.OpenTelemetryKey, span)
	if p.recorder != nil {
		attrs := metric.WithAttributes(semconv.RPCService(r.ServicePath), semconv.RPCMethod(r.ServiceMethod))
		p.recorder.requestsCounter.Add(ctx1, 1, attrs)
		p.recorder.requestSize.Record(ctx1, int64(len(r.Payload)), attrs)
		ctx.(*rc.Context).SetValue(share.OpenTelemetryStartTimeKey, time.Now().UnixMilli())
	}
	ctx.(*rc.Context).Context = ctx1

	return nil
}

func (p OpenTelemetryPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	span := ctx.Value(share.OpenTelemetryKey).(trace.Span)
	defer span.End()

	span.AddEvent(tracingEventRpcxPostWriteResponse, trace.WithAttributes(
		attribute.String(tracingEventRpcxPostWriteResponseMetadata, fmt.Sprintf("%+v", res.Metadata)),
		attribute.String(tracingEventRpcxPostWriteResponsePayload, string(res.Payload)),
	))

	attrs := []attribute.KeyValue{semconv.RPCService(req.ServicePath), semconv.RPCMethod(req.ServiceMethod)}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		attrs = append(attrs, semconv.OTelStatusCodeError)
	} else {
		span.SetStatus(codes.Ok, "success")
		attrs = append(attrs, semconv.OTelStatusCodeOk)
	}

	if p.recorder != nil {
		p.recorder.responsesCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		startTime := ctx.Value(share.OpenTelemetryStartTimeKey).(int64)
		p.recorder.totalDuration.Record(ctx, time.Now().UnixMilli()-startTime, metric.WithAttributes(attrs...))
		p.recorder.responseSize.Record(ctx, int64(len(res.Payload)), metric.WithAttributes(attrs...))
	}

	return nil
}
