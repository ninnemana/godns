package service

import (
	"context"
	"testing"
	"time"

	"github.com/ninnemana/tracelog"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel/metric/metrictest"
)

func TestService_Run(t *testing.T) {
	_, meter := metrictest.NewMeter()
	tests := []struct {
		name    string
		options []Option
		wantErr bool
	}{
		{
			"invalid address",
			[]Option{
				WithLogger(tracelog.NewLogger(tracelog.WithLogger(zap.NewNop()))),
				WithEndpoint(" http://www.google.com"),
				WithMeter(meter),
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := New(tt.options...)
			if err != nil {
				t.Errorf("Run() error = %v", err)
				return
			}

			svc.config = Config{
				Interval: time.Second * 1,
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			if err := svc.Run(ctx); (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
