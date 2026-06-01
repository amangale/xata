package o11y

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ziflex/lecho/v3"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/metadata"

	"xata/internal/api/clienthttpheaders"
	"xata/internal/o11y/version"
)

type TraceIDStyle interface {
	TraceID(trace.TraceID) (string, string)
	SpanID(trace.SpanID) (string, string)
}

type datadogStyle struct{}

var DatadogIDStyle = datadogStyle{}

var internalError = &echo.HTTPError{
	Code:    http.StatusInternalServerError,
	Message: "Internal Error",
}

const (
	tracerName = "xata/o11y/echo"
	tracerKey  = "xata-o11y-echo-tracer"
)

const (
	keyTraceRequestID = "http.request_id"

	keyLogNetRemoteIP    = "network.remote_ip"
	keyLogNetHost        = "network.host"
	keyLogNetBytesRead   = "network.bytes_read"
	keyLogNetBytesWrite  = "network.bytes_written"
	keyLogRequestID      = "http.request_id"
	keyLogHTTPStatusCode = "http.status_code"
	keyLogHTTPMethod     = "http.method"
	keyLogHTTPUserAgent  = "http.user_agent"
	keyLogHTTPURIPath    = "http.url_details.path"
	keyLogHTTPURIRoute   = "http.url_details.route"

	keyLogClientID  = "xata.client_id"
	keyLogSessionID = "xata.session_id"
	keyLogAgent     = "xata.agent"
)

const (
	headerXClientID  = "X-Xata-Client-ID"
	headerXSessionID = "X-Xata-Session-ID"
	headerXAgent     = "X-Xata-Agent"

	// X-Xata-Agent can carry many key=value pairs (client, version, service, cli_command_id,
	// cli_invocation_id, session, ci, pr, ai_agent, …) so we set a max length to accommodate this.
	customHeadersMaxLength = 512
)

func (datadogStyle) TraceID(id trace.TraceID) (string, string) {
	return "dd.trace_id", convertTraceID(id.String())
}

func (datadogStyle) SpanID(id trace.SpanID) (string, string) {
	return "dd.span_id", convertTraceID(id.String())
}

type plainIDStyle struct{}

var PlainIDStyle = plainIDStyle{}

func (plainIDStyle) TraceID(id trace.TraceID) (string, string) {
	return "trace_id", id.String()
}

func (plainIDStyle) SpanID(id trace.SpanID) (string, string) {
	return "span_id", id.String()
}

// LoggerMiddleware replaces the echo/middleware.Logger.
// The middleware installed by loggerMiddleware uses zerolog
// for logging and extends the context with internal request IDs
// to ensure that logs can be correlated well.
func LoggerMiddleware(logger *zerolog.Logger) echo.MiddlewareFunc {
	evtOptString := func(e *zerolog.Event, key, value string) {
		if value != "" {
			e.Str(key, value)
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			req := c.Request()
			res := c.Response()

			ctx := req.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			logCtx := logger.With()

			// Check if requestID was added to the response already and add it to the log message.
			// Note: the middleware.RequestID will read the requestID from the request. If missing
			//       a new ID will be generated and added to the response.
			if requestID := res.Header().Get(echo.HeaderXRequestID); requestID != "" {
				logCtx = logCtx.Str(keyLogRequestID, requestID)
			}

			// Read/validate X-Xata-Client-ID from the request. If missing or invalid, we skip it.
			if clientID := req.Header.Get(headerXClientID); customHeaderValid(clientID) {
				logCtx = logCtx.Str(keyLogClientID, clientID)
			}

			// Read/validate X-Xata-Session-ID from the request. If missing or invalid, we skip it.
			if sessionID := req.Header.Get(headerXSessionID); customHeaderValid(sessionID) {
				logCtx = logCtx.Str(keyLogSessionID, sessionID)
			}

			// Read/validate X-Xata-Agent from the request. If missing or invalid, we skip it.
			if agent := req.Header.Get(headerXAgent); customHeaderValid(agent) {
				logCtx = logCtx.Str(keyLogAgent, agent)
			}

			reqLogger := logCtx.Logger()

			// register API logger with context
			ctx = reqLogger.WithContext(ctx)
			req = req.WithContext(ctx)
			c.SetRequest(req)
			c.SetLogger(lecho.From(reqLogger))

			err := next(c)
			if err != nil {
				c.Error(err)
			}
			stop := time.Now()

			// try to determine the logger for the final log. Other middleware in a sub-group might have
			// replaced the logger with some new logger that carries some more context from other middleware.
			// Let's see if we can find a zerolog logger behind that... If not we will fall back
			// to our logger with the current context.
			var apiLogger *zerolog.Logger
			if lelog, ok := c.Logger().(*lecho.Logger); ok {
				zlog := lelog.Unwrap()
				apiLogger = &zlog
			}
			if logger == nil {
				apiLogger = log.Ctx(ctx)
			}

			e := apiLogger.Info()
			if err != nil || (ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled)) {
				e = apiLogger.Error()
			}

			if e.Enabled() && !isHealthCheck(c) {
				status := res.Status
				if ctxerr := ctx.Err(); ctxerr != nil {
					if err == nil {
						err = ctxerr
					}
					if errors.Is(ctxerr, context.Canceled) { // client did give up
						status = 0
					}
				}

				if status > 0 {
					e.Int(keyLogHTTPStatusCode, status)
				}

				contentLength := 0
				if s := req.Header.Get(echo.HeaderContentLength); s != "" {
					contentLength, _ = strconv.Atoi(s)
				}

				evtOptString(e, keyLogNetRemoteIP, c.RealIP())
				evtOptString(e, keyLogNetHost, hostWithRedactedSlug(NetworkHost(req.Host)))
				evtOptString(e, keyLogHTTPMethod, req.Method)
				evtOptString(e, keyLogHTTPUserAgent, req.UserAgent())
				evtOptString(e, keyLogClientID, req.Header.Get(headerXClientID))
				evtOptString(e, keyLogSessionID, req.Header.Get(headerXSessionID))
				evtOptString(e, keyLogAgent, req.Header.Get(headerXAgent))
				evtOptString(e, keyLogHTTPURIRoute, c.Path())
				if req.URL != nil {
					evtOptString(e, keyLogHTTPURIPath, req.URL.Path)
				}
				if err != nil {
					e.Err(err)
				}

				e.Dur("latency", stop.Sub(start)).
					Int(keyLogNetBytesRead, contentLength).
					Int64(keyLogNetBytesWrite, res.Size).
					Msg("http api request")
			}

			return nil
		}
	}
}

func isHealthCheck(c echo.Context) bool {
	return c.Path() == "/_hello"
}

// RecoverMiddleware recovers from a panic in the API handler currently active.
// The panic + stack trace is logged and an "Internal Error" is returned.
// In comparison to the echo/middleware.Recover middleware we do not
// return an error message with the internal panic message.
func RecoverMiddleware(stackTraceSize int) echo.MiddlewareFunc {
	if stackTraceSize == 0 {
		stackTraceSize = defaultStacktraceSize
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			defer func() {
				if r := recover(); r != nil {
					handleMiddlewareRecover(ctx, log.Ctx(ctx), r, stackTraceSize)
					c.Error(internalError)
				}
			}()

			return next(c)
		}
	}
}

// LoggerWithServiceName adds serviceNamespace and serviceName fields to the logger.
// Use LoggerWithServiceName in routing groups in order to add (or overwrite) the service name
// fields in the logger established by a parent group middleware.
func LoggerWithServiceName(serviceNamespace, serviceName string) echo.MiddlewareFunc {
	return LoggerWithNewCtxMiddleware(func(c echo.Context, lctx zerolog.Context) zerolog.Context {
		return logCtxWithServiceName(lctx, serviceNamespace, serviceName, version.Get())
	})
}

// MetricsMiddleware adds metrics to an echo router or group. It relies on the
// otel http middleware implementation.
func MetricsMiddleware(o *O) echo.MiddlewareFunc {
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// inject the context with the metrics attributes
			httpHandlrFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				labeler, _ := otelhttp.LabelerFromContext(ctx)
				// c.Path from echo library produces the route (path/to/{id}) to
				// avoid exploding the metric by using the full replaced path
				labeler.Add(attribute.KeyValue{Key: "http_route", Value: attribute.StringValue(c.Path())})
				h(c) // call original
			})

			return echo.WrapHandler(otelhttp.NewHandler(httpHandlrFn, o.ServiceName(),
				otelhttp.WithMeterProvider(o),
				otelhttp.WithTracerProvider(noop.NewTracerProvider()),
				otelhttp.WithPropagators(propagation.TraceContext{})))(c)
		}
	}
}

// TracingMiddleware adds tracing support to an echo router or group.
// If a logger has already been set up with the context, then this middleware
// will add tracing ID and span ID to the logger.
func TracingMiddleware(o *O) echo.MiddlewareFunc {
	// Start tracing the request
	tracingMiddleWare := newSpanMiddleware(
		o.ServiceName(),
		o, propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
		o.system.idStyle,
	)

	// Add RequestID to active tracing span
	tracingWithRequestID := withSpanMiddleware(func(c echo.Context, span trace.Span) {
		if requestID := c.Response().Header().Get(echo.HeaderXRequestID); requestID != "" {
			span.SetAttributes(attribute.String(keyTraceRequestID, requestID))
		}
	})

	// Add traceID + spanID to logger if span is active.
	loggerMiddlewareFn := LoggerWithNewCtxMiddleware
	loggerMiddleware := loggerMiddlewareFn(func(c echo.Context, lctx zerolog.Context) zerolog.Context {
		ctx := c.Request().Context()
		spanCtx := trace.SpanContextFromContext(ctx)
		if !spanCtx.HasSpanID() && spanCtx.HasTraceID() {
			return lctx
		}

		if spanCtx.HasSpanID() {
			spanIDKey, spanID := o.system.idStyle.SpanID(spanCtx.SpanID())
			lctx = lctx.Str(spanIDKey, spanID)
		}
		if spanCtx.HasTraceID() {
			traceIDKey, traceID := o.system.idStyle.TraceID(spanCtx.TraceID())
			lctx = lctx.Str(traceIDKey, traceID)
		}

		return lctx
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return tracingMiddleWare(tracingWithRequestID(loggerMiddleware(next)))
	}
}

func newSpanMiddleware(
	service string,
	o *O,
	propagators propagation.TextMapPropagator,
	idStyle TraceIDStyle,
) echo.MiddlewareFunc {
	var provider trace.TracerProvider
	if o == nil {
		provider = otel.GetTracerProvider()
	} else {
		provider = o
	}
	if propagators == nil {
		propagators = otel.GetTextMapPropagator()
	}

	tracer := provider.Tracer(tracerName)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(tracerKey, tracer)

			request := c.Request()
			savedCtx := request.Context()
			defer func() {
				request = request.WithContext(savedCtx)
				c.SetRequest(request)
			}()
			ctx := propagators.Extract(savedCtx, propagation.HeaderCarrier(request.Header))
			opts := []trace.SpanStartOption{
				trace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", request)...),
				trace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(request)...),
				trace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(service, c.Path(), request)...),
				trace.WithSpanKind(trace.SpanKindServer),
			}
			spanName := c.Path()
			if spanName == "" {
				spanName = fmt.Sprintf("HTTP %s route not found", request.Method)
			}

			if o != nil {
				ctx = context.WithValue(ctx, ctxKey{}, o)
			}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer span.End()

			if requestID := c.Response().Header().Get(echo.HeaderXRequestID); requestID != "" {
				span.SetAttributes(attribute.String(keyTraceRequestID, requestID))
				ctx = metadata.AppendToOutgoingContext(ctx, keyLogRequestID, requestID)
			}

			if clientID := c.Request().Header.Get(headerXClientID); customHeaderValid(clientID) {
				span.SetAttributes(attribute.String(keyLogClientID, clientID))
				ctx = metadata.AppendToOutgoingContext(ctx, keyLogClientID, clientID)
			}

			if sessionID := c.Request().Header.Get(headerXSessionID); customHeaderValid(sessionID) {
				span.SetAttributes(attribute.String(keyLogSessionID, sessionID))
				ctx = metadata.AppendToOutgoingContext(ctx, keyLogSessionID, sessionID)
			}

			agentRaw := c.Request().Header.Get(headerXAgent)
			if !customHeaderValid(agentRaw) {
				// don't process invalid X-Xata-Agent headers
				agentRaw = ""
			}
			headers := clienthttpheaders.NewParsedHeaders(c.Request().UserAgent(), agentRaw)
			ctx = clienthttpheaders.NewContext(ctx, headers)
			if agentRaw != "" {
				span.SetAttributes(attribute.String(keyLogAgent, agentRaw))
				if headers.XataAgent.Client != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".client", headers.XataAgent.Client))
				}
				if headers.XataAgent.Version != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".version", headers.XataAgent.Version))
				}
				if headers.XataAgent.Service != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".service", headers.XataAgent.Service))
				}
				if headers.XataAgent.CLICommandID != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".cli_command_id", headers.XataAgent.CLICommandID))
				}
				if headers.XataAgent.CLIInvocationID != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".cli_invocation_id", headers.XataAgent.CLIInvocationID))
				}
				if headers.XataAgent.Session != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".session", headers.XataAgent.Session))
				}
				if headers.XataAgent.CI != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".ci", headers.XataAgent.CI))
				}
				if headers.XataAgent.PR != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".pr", headers.XataAgent.PR))
				}
				if headers.XataAgent.AIAgent != "" {
					span.SetAttributes(attribute.String(keyLogAgent+".ai_agent", headers.XataAgent.AIAgent))
				}
			}

			// pass the span through the request context
			c.SetRequest(request.WithContext(ctx))

			err := next(c)
			if err != nil {
				// register error now so status gets updated
				c.Error(err)
				span.RecordError(err)
			}

			// canceled requests don't throw an error
			if errors.Is(err, context.Canceled) {
				span.SetStatus(codes.Unset, context.Canceled.Error())
				return err
			}

			status := c.Response().Status
			attrs := semconv.HTTPAttributesFromHTTPStatusCode(status)
			span.SetAttributes(attrs...)

			spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(status, trace.SpanKindServer)
			span.SetStatus(spanStatus, spanMessage)

			return err
		}
	}
}

func withSpanMiddleware(fn func(echo.Context, trace.Span)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			span := trace.SpanFromContext(c.Request().Context())
			if span != nil && span.IsRecording() {
				fn(c, span)
			}
			return next(c)
		}
	}
}

// LoggerWithNewCtxMiddleware creates a middleware that will create a new logger
// with a new log context produced by `fn`.
//
// The logger will be passed to the next handler by passing a new `context.Context`.
// The logger installed in the `echo.Context` will be replaced by the new logger.
func LoggerWithNewCtxMiddleware(fn func(echo.Context, zerolog.Context) zerolog.Context) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			RequestLoggerWithCtx(c, fn)
			return next(c)
		}
	}
}

// RequestLoggerWithCtx adds a new logger to the current request context.
// The echo.Contexts request and logger is replaced by a new request and logger.
func RequestLoggerWithCtx(
	c echo.Context,
	fn func(echo.Context, zerolog.Context) zerolog.Context,
) {
	ctx := c.Request().Context()
	logger := fn(c, log.Ctx(ctx).With()).Logger()
	c.SetRequest(c.Request().WithContext(logger.WithContext(ctx)))
	c.SetLogger(lecho.From(logger))
}

func convertTraceID(id string) string {
	if len(id) < 16 {
		return ""
	}
	if len(id) > 16 {
		id = id[16:]
	}
	intValue, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(intValue, 10)
}

func logCtxWithServiceName(lctx zerolog.Context, serviceNamespace, serviceName, serviceVersion string) zerolog.Context {
	const resource = "Resource."
	if serviceNamespace != "" {
		lctx = lctx.Str(resource+string(semconv.ServiceNamespaceKey), serviceNamespace)
	}
	if serviceName != "" {
		lctx = lctx.Str(resource+string(semconv.ServiceNameKey), serviceName)
	}
	if serviceVersion != "" && serviceVersion != "unknown" {
		lctx = lctx.Str(resource+string(semconv.ServiceVersionKey), serviceVersion)
	}
	return lctx
}

// SetReqAttribute includes the given attribute & value into the log context & tracing span of the given request
func SetReqAttribute(c echo.Context, name, value string) {
	RequestLoggerWithCtx(c, func(c echo.Context, lctx zerolog.Context) zerolog.Context {
		lctx = lctx.Str(name, value)
		return lctx
	})
	span := trace.SpanFromContext(c.Request().Context())
	if span != nil && span.IsRecording() {
		span.SetAttributes(attribute.String(name, value))
	}
}

type NetworkHost string

// hostWithRedactedSlug replaces the potentially sensitive slug part from the domain/hostname
// with the string <redacted>.
// the input format is slug-organizationID.xata.io
// the output will be <redacted>-organizationID.xata.io
// In case there is no slug present, the function does not change the input
func hostWithRedactedSlug(host NetworkHost) string {
	hostString := string(host)
	idx := strings.IndexByte(hostString, '.')
	// there is no organization in the networkHost string
	if idx <= 0 {
		return hostString
	}
	organization := hostString[:idx]
	idx = strings.LastIndex(organization, "-")
	// there is no slug in the organization
	if idx < 0 {
		return hostString
	}
	return "<redacted>" + hostString[idx:]
}

func customHeaderValid(id string) bool {
	return len(id) > 0 && len(id) <= customHeadersMaxLength
}
