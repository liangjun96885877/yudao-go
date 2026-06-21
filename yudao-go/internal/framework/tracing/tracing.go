// Package tracing 提供基于 OpenTelemetry 的链路追踪初始化。
// 移植标准 横切能力 #8：OpenTelemetry-Go + W3C TraceContext 跨服务传播。
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider 持有 TracerProvider，用于优雅关闭。
type Provider struct {
	tp *sdktrace.TracerProvider
}

// Init 初始化 OpenTelemetry：span 经 OTLP/HTTP 导出到 Jaeger，
// 并设置全局 W3C TraceContext 传播器（跨服务 traceparent）。
// endpoint 形如 "127.0.0.1:14318"；enabled=false 时仅设置传播器、不导出 span。
func Init(ctx context.Context, serviceName, endpoint string, enabled bool) (*Provider, error) {
	// 传播器始终设置：即便本进程不导出，也要保证入站 traceparent 不丢、能续传。
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	if !enabled {
		return &Provider{}, nil
	}
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return &Provider{tp: tp}, nil
}

// Shutdown 刷新缓冲的 span 并关闭 TracerProvider。
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}
