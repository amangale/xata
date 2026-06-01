package clienthttpheaders

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewParsedHeaders(t *testing.T) {
	tests := map[string]struct {
		userAgent string
		xataAgent string
		want      *ParsedHeaders
	}{
		"empty strings": {
			want: &ParsedHeaders{},
		},
		"user agent only": {
			userAgent: "Mozilla/5.0",
			want:      &ParsedHeaders{UserAgent: "Mozilla/5.0"},
		},
		"single allowed pair": {
			xataAgent: "client=@xata.io/api",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api"}},
		},
		"multiple allowed pairs": {
			xataAgent: "client=@xata.io/api; version=0.1.0; service=console",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "console"}},
		},
		"with cli command": {
			userAgent: "Mozilla/5.0",
			xataAgent: "client=@xata.io/api; version=0.1.0; service=cli; cli_command_id=branch:list; cli_invocation_id=inv-123",
			want:      &ParsedHeaders{UserAgent: "Mozilla/5.0", XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "cli", CLICommandID: "branch:list", CLIInvocationID: "inv-123"}},
		},
		"console with session": {
			xataAgent: "client=@xata.io/api; version=0.1.0; service=console; session=abc123",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "console", Session: "abc123"}},
		},
		"cli with ci and ai_agent": {
			xataAgent: "client=@xata.io/api; version=0.1.0; service=cli; ci=github-actions; pr=true; ai_agent=cursor",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "cli", CI: "github-actions", PR: "true", AIAgent: "cursor"}},
		},
		"no spaces around semicolons": {
			xataAgent: "client=@xata.io/api;version=0.1.0;service=console",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "console"}},
		},
		"extra whitespace": {
			xataAgent: "  client = @xata.io/api ;  version = 0.1.0 ;  service = console  ",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0", Service: "console"}},
		},
		"trailing semicolon": {
			xataAgent: "client=@xata.io/api; version=0.1.0;",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0"}},
		},
		"empty value": {
			xataAgent: "client=; version=0.1.0",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "", Version: "0.1.0"}},
		},
		"missing equals sign skipped": {
			xataAgent: "client=@xata.io/api; garbage; version=0.1.0",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0"}},
		},
		"empty key skipped": {
			xataAgent: "=value; client=@xata.io/api",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api"}},
		},
		"unknown fields filtered out": {
			xataAgent: "client=@xata.io/api; unknown=foo; version=0.1.0",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Version: "0.1.0"}},
		},
		"only unknown fields": {
			xataAgent: "unknown=foo; bar=baz",
			want:      &ParsedHeaders{},
		},
		"value with equals sign": {
			xataAgent: "client=@xata.io/api; service=a=b",
			want:      &ParsedHeaders{XataAgent: ParsedXataAgent{Client: "@xata.io/api", Service: "a=b"}},
		},
		"only semicolons": {
			xataAgent: ";;;",
			want:      &ParsedHeaders{},
		},
		"only whitespace": {
			xataAgent: "   ",
			want:      &ParsedHeaders{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewParsedHeaders(tt.userAgent, tt.xataAgent)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestContext(t *testing.T) {
	t.Run("round-trips through context", func(t *testing.T) {
		headers := NewParsedHeaders("Mozilla/5.0", "client=@xata.io/api; version=0.1.0")
		ctx := NewContext(context.Background(), headers)
		got := FromContext(ctx)
		require.Equal(t, headers, got)
	})

	t.Run("returns nil for missing headers", func(t *testing.T) {
		got := FromContext(context.Background())
		require.Nil(t, got)
	})
}
