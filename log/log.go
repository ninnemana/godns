package log

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return c.Logger
	}

	log := c.Logger.With(
		zap.String("traceID", span.SpanContext().TraceID().String()),
		zap.String("spanID", span.SpanContext().SpanID().String()),
	)

	span.AddEvent(msg)

	var attrs []attribute.KeyValue
	for _, field := range fields {
		switch field.Type {
		case zapcore.NamespaceType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.UnknownType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.SkipType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.ArrayMarshalerType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.ObjectMarshalerType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.BinaryType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.BoolType:
			attrs = append(attrs, attribute.Bool(field.Key, field.Integer == 1))
		case zapcore.ByteStringType:
			bits, ok := field.Interface.([]byte)
			if ok {
				attrs = append(attrs, attribute.String(field.Key, string(bits)))
			}
		case zapcore.Complex128Type:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.Complex64Type:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.DurationType:
			attrs = append(attrs, attribute.String(field.Key, time.Duration(field.Integer).String()))
		case zapcore.Float64Type:
			attrs = append(attrs, attribute.Float64(field.Key, float64(field.Integer)))
		case zapcore.Float32Type:
			attrs = append(attrs, attribute.Float64(field.Key, float64(field.Integer)))
		case zapcore.Int16Type, zapcore.Int8Type, zapcore.Int64Type:
			attrs = append(attrs, attribute.Int64(field.Key, int64(field.Integer)))
		case zapcore.Int32Type:
			attrs = append(attrs, attribute.Int64(field.Key, field.Integer))
		case zapcore.StringType:
			attrs = append(attrs, attribute.String(field.Key, field.String))
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
				attrs = append(attrs, attribute.String(field.Key, t.String()))
			}
		case zapcore.ReflectType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.StringerType:
			attrs = append(attrs, attribute.Any(field.Key, field.Interface))
		case zapcore.ErrorType:
			span.RecordError(field.Interface.(error))
		}
	}

	span.SetAttributes(attrs...)

	return log
}
