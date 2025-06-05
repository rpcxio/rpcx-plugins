package share

import (
	"context"
	"sync"

	"github.com/smallnest/rpcx/share"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type OpenTelemetryKeyType int

const (
	OpenTelemetryKey OpenTelemetryKeyType = iota
	OpenTelemetryStartTimeKey
)

type metadataSupplier struct {
	metadata map[string]string
	lock     *sync.RWMutex
}

func newMetadataSupplier(mp map[string]string) *metadataSupplier {
	return &metadataSupplier{metadata: mp, lock: new(sync.RWMutex)}
}

var _ propagation.TextMapCarrier = newMetadataSupplier(make(map[string]string))

func (s *metadataSupplier) Get(key string) string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.metadata[key]
}

func (s *metadataSupplier) Set(key string, value string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.metadata[key] = value
}

func (s *metadataSupplier) Keys() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	out := make([]string, 0, len(s.metadata))
	for key := range s.metadata {
		out = append(out, key)
	}
	return out
}

func Inject(ctx context.Context, propagators propagation.TextMapPropagator) {
	meta := ctx.Value(share.ReqMetaDataKey)
	if meta == nil {
		meta = make(map[string]string)
		if rpcxContext, ok := ctx.(*share.Context); ok {
			rpcxContext.SetValue(share.ReqMetaDataKey, meta)
		}
	}

	propagators.Inject(ctx, newMetadataSupplier(meta.(map[string]string)))
}

func Extract(ctx context.Context, propagators propagation.TextMapPropagator) trace.SpanContext {
	meta := ctx.Value(share.ReqMetaDataKey)
	if meta == nil {
		meta = make(map[string]string)
		if rpcxContext, ok := ctx.(*share.Context); ok {
			rpcxContext.SetValue(share.ReqMetaDataKey, meta)
		}
	}

	ctx = propagators.Extract(ctx, newMetadataSupplier(meta.(map[string]string)))

	return trace.SpanContextFromContext(ctx)
}
