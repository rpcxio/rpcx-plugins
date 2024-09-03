package otel

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"
	rc "github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
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
	tracingCommonKeyIpIntranet                = `ip.intranet`
	tracingCommonKeyIpHostname                = `hostname`
	tracingEventRpcxPreHandleRequest          = "rpcx.pre.handle.request"
	tracingEventRpcxPreHandleRequestPath      = "rpcx.pre.handle.request.path"
	tracingEventRpcxPreHandleRequestMetadata  = "rpcx.pre.handle.request.metadata"
	tracingEventRpcxPreHandleRequestPayload   = "rpcx.pre.handle.request.payload"
	tracingEventRpcxPostWriteResponse         = "rpcx.post.write.response"
	tracingEventRpcxPostWriteResponseMetadata = "rpcx.post.write.response.metadata"
	tracingEventRpcxPostWriteResponsePayload  = "rpcx.post.write.response.payload"
	metricRequestPath                         = "rpcx.client.request.path"
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
	p.recorder = GetRecorder(meter, "")
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
	var span trace.Span
	shareContext := ctx.(*rc.Context).Context

	spanName := fmt.Sprintf("rpcx.service.%s.%s", r.ServicePath, r.ServiceMethod)
	shareContext, span = p.tracer.Start(
		shareContext,
		spanName,
		trace.WithSpanKind(trace.SpanKindServer),
	)
	hostname, _ := os.Hostname()
	intranetIps, _ := GetIntranetIpArray()
	intranetIpStr := strings.Join(intranetIps, ",")
	span.SetAttributes(
		attribute.String(tracingCommonKeyIpHostname, hostname),
		attribute.String(tracingCommonKeyIpIntranet, intranetIpStr),
		semconv.HostName(hostname))
	shareContext = context.WithValue(shareContext, "RpcXServerTracingHandled", 1)

	span.AddEvent(tracingEventRpcxPreHandleRequest, trace.WithAttributes(
		attribute.String(tracingEventRpcxPreHandleRequestPath, spanName),
		attribute.String(tracingEventRpcxPreHandleRequestMetadata, fmt.Sprintf("%+v", r.Metadata)),
		attribute.String(tracingEventRpcxPreHandleRequestPayload, string(r.Payload)),
	))
	ctx.(*rc.Context).SetValue(share.OpenTelemetryKey, span)
	if p.recorder != nil {
		p.recorder.activeRequestsCounter.Add(shareContext, 1, metric.WithAttributes(attribute.String(metricRequestPath, spanName)))
		ctx.(*rc.Context).SetValue(share.OpenTelemetryStartTimeKey, time.Now().UnixMilli())
	}
	ctx.(*rc.Context).Context = shareContext

	return nil
}

func (p OpenTelemetryPlugin) PostWriteResponse(ctx context.Context, req *protocol.Message, res *protocol.Message, err error) error {
	span := ctx.Value(share.OpenTelemetryKey).(trace.Span)
	defer span.End()

	span.AddEvent(tracingEventRpcxPostWriteResponse, trace.WithAttributes(
		attribute.String(tracingEventRpcxPostWriteResponseMetadata, fmt.Sprintf("%+v", res.Metadata)),
		attribute.String(tracingEventRpcxPostWriteResponsePayload, string(res.Payload)),
	))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "success")
	}

	spanName := fmt.Sprintf("rpcx.service.%s.%s", req.ServicePath, req.ServiceMethod)

	attrs := metric.WithAttributes(attribute.String(metricRequestPath, spanName))
	if p.recorder != nil {
		p.recorder.activeRequestsCounter.Add(ctx, -1, attrs)
		p.recorder.attemptsCounter.Add(ctx, 1, attrs)
		startTime := ctx.Value(share.OpenTelemetryStartTimeKey).(int64)
		p.recorder.totalDuration.Record(ctx, time.Now().UnixMilli()-startTime, attrs)
		p.recorder.requestSize.Record(ctx, int64(len(req.Payload)), attrs)
		p.recorder.responseSize.Record(ctx, int64(len(res.Payload)), attrs)
	}

	return nil
}

func GetIntranetIpArray() (ips []string, err error) {
	var (
		addresses  []net.Addr
		interFaces []net.Interface
	)
	interFaces, err = net.Interfaces()
	if err != nil {
		return ips, err
	}
	for _, interFace := range interFaces {
		if interFace.Flags&net.FlagUp == 0 {
			// interface down
			continue
		}
		if interFace.Flags&net.FlagLoopback != 0 {
			// loop back interface
			continue
		}
		// ignore warden bridge
		if strings.HasPrefix(interFace.Name, "w-") {
			continue
		}
		addresses, err = interFace.Addrs()
		if err != nil {
			return ips, err
		}
		for _, addr := range addresses {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				// not an ipv4 address
				continue
			}
			ipStr := ip.String()
			if IsIntranet(ipStr) {
				ips = append(ips, ipStr)
			}
		}
	}
	return ips, nil
}

// IsIntranet checks and returns whether given ip an intranet ip.
//
// Local: 127.0.0.1
// A: 10.0.0.0--10.255.255.255
// B: 172.16.0.0--172.31.255.255
// C: 192.168.0.0--192.168.255.255
func IsIntranet(ip string) bool {
	if ip == "127.0.0.1" {
		return true
	}
	array := strings.Split(ip, ".")
	if len(array) != 4 {
		return false
	}
	// A
	if array[0] == "10" || (array[0] == "192" && array[1] == "168") {
		return true
	}
	// C
	if array[0] == "192" && array[1] == "168" {
		return true
	}
	// B
	if array[0] == "172" {
		second, err := strconv.ParseInt(array[1], 10, 64)
		if err != nil {
			return false
		}
		if second >= 16 && second <= 31 {
			return true
		}
	}
	return false
}
