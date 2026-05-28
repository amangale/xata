package o11y

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/elastic/go-concert/ctxtool"
	"github.com/elastic/go-concert/timed"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"

	"xata/internal/idgen"
	"xata/internal/o11y/version"
)

type metrics struct {
	out             metricExporter
	defaultResource *resource.Resource
	logger          *zerolog.Logger

	controllersStop ctxtool.CancelContext
	chDone          chan struct{}
	controllers     metricsControllerList
}

type metricExporter interface {
	sdkmetric.Exporter
	Shutdown(context.Context) error
}

type metricsControllerList struct {
	mu      sync.Mutex
	list    []*metricsController
	idx     map[string]int
	dropped map[string]struct{}
	active  bool
}

type metricsController struct {
	embedded.MeterProvider

	id       string
	reader   sdkmetric.Reader
	provider *sdkmetric.MeterProvider
	resource *resource.Resource
}

func initMetrics(
	ctx context.Context,
	logger *zerolog.Logger,
	res *resource.Resource,
	period time.Duration,
) *metrics {
	if period <= 0 {
		return nil
	}

	metricsLogger := logger.With().Str("component", "metrics").Logger()

	metricsExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithTemporalitySelector(deltaSelector),
		otlpmetricgrpc.WithDialOption(
			grpc.WithUnaryInterceptor(GRPCLoggingUnaryClientInterceptor(&metricsLogger)),
			grpc.WithStreamInterceptor(GRPCLoggingStreamClientInterceptor(&metricsLogger)),
		),
	)
	if err != nil {
		return nil
	}

	m := &metrics{
		out:             metricsExporter,
		logger:          logger,
		defaultResource: res,
		controllersStop: ctxtool.WithCancelContext(context.Background()),
		chDone:          make(chan struct{}, 1),
		controllers: metricsControllerList{
			idx:     map[string]int{},
			dropped: map[string]struct{}{},
		},
	}

	go m.ticker(period)

	return m
}

func (m *metrics) ticker(period time.Duration) {
	defer func() { m.chDone <- struct{}{} }()

	err := timed.Periodic(m.controllersStop, period, func() error {
		m.collect(m.controllersStop)
		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		m.logger.Err(err).Msg("metrics controller failed")
	}
}

func (m *metrics) shutdown(ctx context.Context) (err error) {
	if m == nil {
		return nil
	}

	m.controllersStop.Cancel()
	if m.out != nil {
		err = m.out.Shutdown(ctx)
	}

	select {
	case <-m.chDone:
	case <-ctx.Done():
		if err == nil {
			err = ctx.Err()
		}
	}

	return err
}

func (m *metrics) collect(ctx context.Context) {
	list := func() []*metricsController {
		m.controllers.mu.Lock()
		defer m.controllers.mu.Unlock()
		m.controllers.active = true
		return m.controllers.list
	}()

	defer func() {
		m.controllers.mu.Lock()
		defer m.controllers.mu.Unlock()
		m.controllers.active = false
		m.unregisterDroppedNoLock()
	}()

	for i := 0; i < len(list) && ctx.Err() == nil; i++ {
		_ = list[i].CollectAndExport(ctx, m.out)
	}
}

func (m *metrics) Provider(
	serviceNamespace, serviceName string,
) metric.MeterProvider {
	if m == nil {
		return noop.NewMeterProvider()
	}

	id := idgen.Generate()

	serviceResource := resource.NewSchemaless(
		semconv.ServiceNameKey.String(fmt.Sprintf("%s_%s", serviceNamespace, serviceName)),
		semconv.ServiceVersionKey.String(version.Get()),
	)

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(serviceResource),
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(
			sdkmetric.NewView(
				sdkmetric.Instrument{
					Scope: instrumentation.Scope{Name: "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"},
				},
				sdkmetric.Stream{
					AttributeFilter: attribute.NewDenyKeysFilter("net.sock.peer.port", "net.sock.peer.addr"),
				},
			),
		),
	)

	mc := &metricsController{
		id:       id,
		reader:   reader,
		provider: provider,
		resource: serviceResource,
	}
	m.register(mc)
	otel.SetMeterProvider(provider)

	return mc
}

func (m *metrics) register(mc *metricsController) {
	m.controllers.mu.Lock()
	defer m.controllers.mu.Unlock()

	if !m.controllers.active {
		m.unregisterDroppedNoLock()
	}

	idx := len(m.controllers.list)
	m.controllers.list = append(m.controllers.list, mc)
	m.controllers.idx[mc.id] = idx
}

func (m *metrics) unregister(mp metric.MeterProvider) {
	mc, ok := mp.(*metricsController)
	if !ok {
		return
	}

	m.controllers.mu.Lock()
	defer m.controllers.mu.Unlock()

	if m.controllers.active {
		m.controllers.dropped[mc.id] = struct{}{}
		return
	}

	m.unregisterDroppedNoLock()
	m.unregisterWithIDNoLock(mc.id)
}

func (m *metrics) unregisterDroppedNoLock() {
	if len(m.controllers.dropped) == 0 {
		return
	}

	for id := range m.controllers.dropped {
		m.unregisterWithIDNoLock(id)
	}
	m.controllers.dropped = make(map[string]struct{})
}

func (m *metrics) unregisterWithIDNoLock(id string) {
	idx, exists := m.controllers.idx[id]
	if !exists {
		return
	}

	// remove controller by replacing it with the last one
	L := len(m.controllers.list) - 1
	tmp := m.controllers.list[L]
	m.controllers.list[idx] = tmp
	m.controllers.list[L] = nil
	m.controllers.list = m.controllers.list[:L]

	// update index
	m.controllers.idx[tmp.id] = idx
	delete(m.controllers.idx, id)
}

func (c *metricsController) Meter(instrumentationName string, opts ...metric.MeterOption) metric.Meter {
	return c.provider.Meter(instrumentationName, opts...)
}

func (c *metricsController) CollectAndExport(ctx context.Context, exporter sdkmetric.Exporter) error {
	rm := metricdata.ResourceMetrics{}
	err := c.reader.Collect(ctx, &rm)
	if err != nil {
		return err
	}
	return exporter.Export(ctx, &rm)
}

// OpenTelemetry protocol supports two ways of representing metrics in time:
// Cumulative and Delta temporality. This function sets the temporality
// preference of the OpenTelemetry implementation to DELTA, because setting it
// to CUMULATIVE may discard some data points during application (or collector)
// startup.
//
// https://docs.datadoghq.com/opentelemetry/guide/otlp_delta_temporality/?code-lang=go#configuring-your-opentelemetry-sdk

func deltaSelector(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	switch kind {
	case sdkmetric.InstrumentKindCounter,
		sdkmetric.InstrumentKindHistogram,
		sdkmetric.InstrumentKindObservableGauge,
		sdkmetric.InstrumentKindObservableCounter:
		return metricdata.DeltaTemporality
	case sdkmetric.InstrumentKindUpDownCounter,
		sdkmetric.InstrumentKindObservableUpDownCounter:
		return metricdata.CumulativeTemporality
	default:
		panic("unknown instrument kind")
	}
}
