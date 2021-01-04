package log

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracelog "github.com/opentracing/opentracing-go/log"
	"github.com/uber/jaeger-client-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Contextual struct {
	*zap.Logger
}

func (c *Contextual) Error(ctx context.Context, msg string, fields ...zap.Field) {
	l := c.trace(ctx, msg, fields...)

	l.Error(msg, fields...)
}

func (c *Contextual) Info(ctx context.Context, msg string, fields ...zap.Field) {
	l := c.trace(ctx, msg, fields...)

	l.Info(msg, fields...)
}

func (c *Contextual) trace(ctx context.Context, msg string, fields ...zap.Field) *zap.Logger {
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return c.Logger
	}

	log := c.Logger.With(
		zap.String("traceID", traceID(span)),
		zap.String("spanID", spanID(span)),
	)

	span.LogFields(tracelog.String("msg", msg))

	for _, field := range fields {
		switch field.Type {
		case zapcore.NamespaceType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.UnknownType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.SkipType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.ArrayMarshalerType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.ObjectMarshalerType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.BinaryType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.BoolType:
			span.SetTag(field.Key, field.Integer == 1)
		case zapcore.ByteStringType:
			bits, ok := field.Interface.([]byte)
			if ok {
				span.SetTag(field.Key, string(bits))
			}
		case zapcore.Complex128Type:
			span.SetTag(field.Key, field.Interface)
		case zapcore.Complex64Type:
			span.SetTag(field.Key, field.Interface)
		case zapcore.DurationType:
			span.SetTag(field.Key, time.Duration(field.Integer))
		case zapcore.Float64Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Float32Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Int64Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Int32Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Int16Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Int8Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.StringType:
			span.SetTag(field.Key, field.String)
		case zapcore.TimeType:
			loc, ok := field.Interface.(*time.Location)
			if !ok {
				break
			}

			t, err := time.ParseInLocation(
				time.RFC3339Nano,
				time.Unix(0, field.Integer).Format(time.RFC3339Nano),
				loc,
			)
			if err == nil {
				span.SetTag(field.Key, t.String())
			}
		case zapcore.Uint64Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Uint32Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Uint16Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.Uint8Type:
			span.SetTag(field.Key, field.Integer)
		case zapcore.UintptrType:
			span.SetTag(field.Key, field.Integer)
		case zapcore.ReflectType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.StringerType:
			span.SetTag(field.Key, field.Interface)
		case zapcore.ErrorType:
			span.SetTag("error.message", field.Interface.(error))
			ext.Error.Set(span, true)
		}
	}

	return log
}

func traceID(sp opentracing.Span) string {
	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok {
		return ""
	}

	return sctx.TraceID().String()
}

func spanID(sp opentracing.Span) string {
	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok {
		return ""
	}

	return sctx.SpanID().String()
}
