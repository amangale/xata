package o11y

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"xata/internal/api/clienthttpheaders"
)

func TestTracingMiddlewareClientHeadersContext(t *testing.T) {
	tests := map[string]struct {
		userAgent string
		xataAgent string
		want      *clienthttpheaders.ParsedHeaders
	}{
		"no headers": {
			want: &clienthttpheaders.ParsedHeaders{},
		},
		"user agent only": {
			userAgent: "curl/8.7.1",
			want:      &clienthttpheaders.ParsedHeaders{UserAgent: "curl/8.7.1"},
		},
		"xata agent only": {
			xataAgent: "client=@xata.io/api; version=0.1.0; service=cli",
			want: &clienthttpheaders.ParsedHeaders{
				XataAgent: clienthttpheaders.ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "cli"},
			},
		},
		"user agent with invalid xata agent": {
			userAgent: "curl/8.7.1",
			xataAgent: "client=" + strings.Repeat("a", customHeadersMaxLength),
			want:      &clienthttpheaders.ParsedHeaders{UserAgent: "curl/8.7.1"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got *clienthttpheaders.ParsedHeaders
			handler := newSpanMiddleware("test", nil, nil, PlainIDStyle)(func(c echo.Context) error {
				got = clienthttpheaders.FromContext(c.Request().Context())
				return c.NoContent(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}
			if tt.xataAgent != "" {
				req.Header.Set(headerXAgent, tt.xataAgent)
			}

			e := echo.New()
			c := e.NewContext(req, httptest.NewRecorder())
			c.SetPath("/test")

			err := handler(c)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
