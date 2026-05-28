package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"time"

	"xata/internal/o11y"
	"xata/services/gateway/initiator"
	"xata/services/gateway/metrics"
	"xata/services/gateway/session"

	"github.com/elastic/go-concert/ctxtool"
	"github.com/google/uuid"
	proxyproto "github.com/pires/go-proxyproto"
	"github.com/rs/zerolog/log"
)

type Server interface {
	Run(context.Context) error
}

// Server implements the SQL wire protocol server. The server uses the
// `Initiator` to accept and configure a new `Session` when a client connects.
type server struct {
	initiator sessionInitiator
	drainer   *timedWaitGroup
	listenURL string
	metrics   *metrics.GatewayMetrics
}

// Initiator creates and configures a new session. The Initiator should handle
// authentication before creating an actual session.
type sessionInitiator interface {
	InitSession(ctx context.Context, sessionID string, conn net.Conn) (session.Session, error)
}

const sessionIDKey = "session_id"

// NewServer creates a new SQL gateway server.
func NewServer(si sessionInitiator, cfg ServerConfig, m *metrics.GatewayMetrics) Server {
	if si == nil {
		si = initiator.NewRejectInitiator()
	}

	return &server{
		listenURL: cfg.Listen,
		drainer:   newTimedWaitGroup(cfg.DrainingTime),
		initiator: si,
		metrics:   m,
	}
}

func (s *server) Run(ctx context.Context) error {
	var ac ctxtool.AutoCancel
	defer ac.Cancel()

	lc := net.ListenConfig{}
	baseListener, err := lc.Listen(ctx, "tcp", s.listenURL)
	if err != nil {
		return err
	}

	// Wrap the listener with PROXY protocol support
	// This enables automatic detection and parsing of PROXY protocol v1 and v2 headers
	// If PROXY protocol headers are present, RemoteAddr() will return the real client IP
	// If not present, it falls back to the actual connection RemoteAddr()
	proxyListener := &proxyproto.Listener{
		Listener: baseListener,
		ConnPolicy: func(opts proxyproto.ConnPolicyOptions) (proxyproto.Policy, error) {
			return proxyproto.USE, nil
		},
	}
	log.Info().Msgf("listening on %s with PROXY protocol support...", s.listenURL)

	// Close listener and give connections time to finish
	ctx = ac.With(ctxtool.WithFunc(ctx, func() {
		proxyListener.Close()

		if err := s.drainer.Wait(context.Background()); err != nil {
			// Check if it was a timeout by looking at the error type
			if ctx.Err() == nil {
				// It was a draining timeout, not context cancellation
				log.Info().Msgf("draining period completed, closing %v active connections after draining period",
					s.drainer.GetCount())
			}
		} else {
			log.Info().Msg("no active connections left")
		}
	}))

	return s.serve(ctx, proxyListener)
}

func (s *server) serve(ctx context.Context, l net.Listener) error {
	o := o11y.Ctx(ctx)
	logger := o.Logger()

	for ctx.Err() == nil {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					event := logger.Error().Bytes("error.stack", debug.Stack())
					if err, ok := r.(error); ok {
						event = event.Err(err)
					} else {
						event = event.Err(fmt.Errorf("%v", r))
					}
					event.Msg("session panic")
				}
			}()

			defer conn.Close()

			startTime := time.Now()
			s.metrics.ConnectionStart(ctx, metrics.ProtocolWire)
			var branchID string
			defer func() {
				s.metrics.ConnectionEnd(ctx, metrics.ProtocolWire, time.Since(startTime))
			}()

			sessionID := uuid.New().String()
			sessionLogger := logger.With().Str(sessionIDKey, sessionID).Logger()

			sessionLogger.Debug().Msg("new client session")
			defer func() { sessionLogger.Debug().Msg("client session ended") }()

			var err error
			branchID, err = s.startSession(sessionLogger.WithContext(ctx), sessionID, conn)

			// Enrich after startSession: RemoteAddr()/ProxyHeader() on a
			// proxyproto.Conn must not be called before peekByte (see initiator).
			sessionLogger = sessionLogger.With().
				Str("client_addr", conn.RemoteAddr().String()).
				Bool("external_connection", isProxyProtocolConn(conn)).
				Str("branch_id", branchID).
				Logger()

			if err != nil {
				if isIgnorableError(err) {
					sessionLogger.Debug().Err(err).Msg("ignorable error during gw session")
				} else {
					sessionLogger.Error().Err(err).Msg("error during gw session")
				}
			}
		}()
	}

	return ctx.Err()
}

func isProxyProtocolConn(conn net.Conn) bool {
	proxyConn, ok := conn.(*proxyproto.Conn)
	if !ok {
		return false
	}
	return proxyConn.ProxyHeader() != nil
}

func isIgnorableError(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, initiator.ErrorSSLRequired) ||
		errors.Is(err, initiator.ErrorStartupMsgCode) ||
		errors.Is(err, initiator.ErrorStartupMsgLength)
}

func (s *server) startSession(ctx context.Context, sessionID string, clientConn net.Conn) (string, error) {
	if err := s.drainer.Add(1); err != nil {
		clientConn.Close()
		log.Info().Msg("connection attempt rejected while in draining mode")
		return "", nil
	}

	ctx, cancel := ctxtool.WithFunc(ctx, func() {
		defer s.drainer.Done()
		if err := clientConn.Close(); err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Debug().Msgf("connection was already closed")
			} else {
				log.Err(err).Msgf("close client connection")
			}
		}
	})
	defer cancel()

	session, err := s.initiator.InitSession(ctx, sessionID, clientConn)
	if err != nil {
		return "", err
	}
	return session.BranchID(), session.ServeSQLSession(ctx)
}
