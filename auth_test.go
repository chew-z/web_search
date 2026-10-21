package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	json "encoding/json/v2"

	"github.com/chew-z/web_search/internal/oauth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestExtractTokenFromHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "valid_bearer",
			header: "Bearer token-123",
			want:   "token-123",
		},
		{
			name:   "missing_bearer_prefix",
			header: "Token token-123",
			want:   "",
		},
		{
			name:   "wrong_scheme",
			header: "Basic abc123",
			want:   "",
		},
		{
			name:   "empty_string",
			header: "",
			want:   "",
		},
		{
			name:   "extra_whitespace",
			header: "  Bearer  two-spaces  ",
			want:   "two-spaces",
		},
		{
			name:   "multiple_spaces",
			header: "Bearer      token-with-multiple-spaces",
			want:   "token-with-multiple-spaces",
		},
		{
			name:   "only_bearer",
			header: "Bearer",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractTokenFromHeader(tt.header)
			if got != tt.want {
				t.Fatalf("extractTokenFromHeader(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

// signTestToken builds a signed JWT string for the guard tests.
func signTestToken(t *testing.T, secret string, claims Claims, method *jwt.SigningMethodHMAC) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// bearerRequest returns a POST request carrying the given Authorization header value.
func bearerRequest(t *testing.T, header string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com/mcp", http.NoBody)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	return req
}

func TestSignTokenRoundTrip(t *testing.T) {
	t.Parallel()

	secret := "round-trip-secret"

	t.Run("generate_static_token", func(t *testing.T) {
		t.Parallel()
		token, err := GenerateStaticToken(secret, "alice", time.Hour)
		if err != nil {
			t.Fatalf("GenerateStaticToken: %v", err)
		}
		claims, err := parseBearerToken(secret, bearerRequest(t, "Bearer "+token))
		if err != nil {
			t.Fatalf("parseBearerToken: %v", err)
		}
		if claims.Issuer != staticIssuer || !hasAudience(claims.Audience, staticAudience) {
			t.Fatalf("claims = iss %q aud %v", claims.Issuer, claims.Audience)
		}
		if claims.Username != "alice" || claims.Scope != "" {
			t.Fatalf("claims = username %q scope %q", claims.Username, claims.Scope)
		}
	})

	t.Run("ttl_must_be_positive", func(t *testing.T) {
		t.Parallel()
		if _, err := SignToken(secret, staticIssuer, staticAudience, "alice", "", 0); err == nil {
			t.Fatal("SignToken with zero TTL should fail")
		}
	})

	t.Run("hs256_signer_carries_scope", func(t *testing.T) {
		t.Parallel()
		signer := HS256Signer{Secret: secret, Issuer: "https://auth.test/answer", Audience: "https://api.test/answer/mcp"}
		token, err := signer.Sign("bob", "answer", time.Hour)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		claims, err := parseBearerToken(secret, bearerRequest(t, "Bearer "+token))
		if err != nil {
			t.Fatalf("parseBearerToken: %v", err)
		}
		if claims.Issuer != "https://auth.test/answer" || claims.Scope != "answer" || claims.Username != "bob" {
			t.Fatalf("claims = %+v", claims)
		}
	})
}

func TestParseBearerToken(t *testing.T) {
	t.Parallel()

	now := time.Now()
	secret := "test-secret"
	wrongSecret := "wrong-secret"

	baseClaims := func() Claims {
		return Claims{
			UserID:    "u-1",
			Username:  "alice",
			Role:      "user",
			Issuer:    staticIssuer,
			Audience:  jwt.ClaimStrings{staticAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}
	}

	tests := []struct {
		name        string
		token       string
		wantErr     bool
		wantErrType error
	}{
		{
			name:    "valid_hs256_token",
			token:   signTestToken(t, secret, baseClaims(), jwt.SigningMethodHS256),
			wantErr: false,
		},
		{
			name: "expired_token",
			token: func() string {
				c := baseClaims()
				c.ExpiresAt = jwt.NewNumericDate(now.Add(-2 * time.Minute))
				c.IssuedAt = jwt.NewNumericDate(now.Add(-3 * time.Minute))
				return signTestToken(t, secret, c, jwt.SigningMethodHS256)
			}(),
			wantErr:     true,
			wantErrType: jwt.ErrTokenExpired,
		},
		{
			name: "missing_exp_is_rejected",
			token: func() string {
				c := baseClaims()
				c.ExpiresAt = nil
				return signTestToken(t, secret, c, jwt.SigningMethodHS256)
			}(),
			wantErr: true,
		},
		{
			name: "empty_username_is_rejected",
			token: func() string {
				c := baseClaims()
				c.Username = "  "
				return signTestToken(t, secret, c, jwt.SigningMethodHS256)
			}(),
			wantErr: true,
		},
		{
			name:        "wrong_signing_method",
			token:       signTestToken(t, secret, baseClaims(), jwt.SigningMethodHS384),
			wantErr:     true,
			wantErrType: jwt.ErrTokenSignatureInvalid,
		},
		{
			name:    "malformed_token_string",
			token:   "not-a-jwt-token",
			wantErr: true,
		},
		{
			name:    "invalid_signature",
			token:   signTestToken(t, wrongSecret, baseClaims(), jwt.SigningMethodHS256),
			wantErr: true,
		},
		{
			name:    "missing_header",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := ""
			if tt.token != "" {
				header = "Bearer " + tt.token
			}
			claims, err := parseBearerToken(secret, bearerRequest(t, header))
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("parseBearerToken() unexpected error = %v", err)
				}
				if claims == nil || claims.Username == "" {
					t.Fatalf("parseBearerToken() got empty claims")
				}
				return
			}
			if err == nil {
				t.Fatalf("parseBearerToken() expected error")
			}
			if tt.wantErrType != nil && !errors.Is(err, tt.wantErrType) {
				t.Fatalf("parseBearerToken() error = %v, want wrapped %v", err, tt.wantErrType)
			}
		})
	}
}

// staticChallenge is the WWW-Authenticate value expected when OAuth is not configured.
const staticChallenge = `Bearer realm="answer-mcp", error="invalid_token"`

func TestProtectMCP(t *testing.T) {
	t.Parallel()

	secret := "test-secret"
	now := time.Now()

	makeClaimsToken := func(issuer, audience string, exp time.Time) string {
		t.Helper()
		return signTestToken(t, secret, Claims{
			UserID:    "u-42",
			Username:  "alice",
			Role:      "admin",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		}, jwt.SigningMethodHS256)
	}

	tests := []struct {
		name          string
		method        string
		setupHeader   func() string
		wantStatus    int
		wantChallenge string
		wantBody      string
		expectContext bool
	}{
		{
			name:   "valid_static_token_reaches_handler_with_user_info",
			method: http.MethodPost,
			setupHeader: func() string {
				return "Bearer " + makeClaimsToken(staticIssuer, staticAudience, now.Add(time.Hour))
			},
			wantStatus:    http.StatusOK,
			expectContext: true,
		},
		{
			name:          "missing_authorization_header",
			method:        http.MethodPost,
			setupHeader:   func() string { return "" },
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: staticChallenge,
			wantBody:      `{"error":"unauthorized"}`,
		},
		{
			name:          "malformed_token",
			method:        http.MethodPost,
			setupHeader:   func() string { return "Bearer not-a-jwt-token" },
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: staticChallenge,
			wantBody:      `{"error":"unauthorized"}`,
		},
		{
			name:   "expired_token",
			method: http.MethodPost,
			setupHeader: func() string {
				return "Bearer " + makeClaimsToken(staticIssuer, staticAudience, now.Add(-2*time.Minute))
			},
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: staticChallenge,
			wantBody:      `{"error":"unauthorized"}`,
		},
		{
			name:   "unknown_issuer",
			method: http.MethodPost,
			setupHeader: func() string {
				return "Bearer " + makeClaimsToken("someone-else", staticAudience, now.Add(time.Hour))
			},
			wantStatus:    http.StatusUnauthorized,
			wantChallenge: staticChallenge,
			wantBody:      `{"error":"unauthorized"}`,
		},
		{
			name:          "options_passes_preflight_without_auth",
			method:        http.MethodOptions,
			setupHeader:   func() string { return "" },
			wantStatus:    http.StatusOK,
			wantChallenge: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedCtx context.Context
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedCtx = r.Context()
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			})
			guard := newAuthGuard(secret, nil)

			req := httptest.NewRequestWithContext(t.Context(), tt.method, "http://example.com/mcp", http.NoBody)
			if header := tt.setupHeader(); header != "" {
				req.Header.Set("Authorization", header)
			}

			rec := httptest.NewRecorder()
			guard.protectMCP(next).ServeHTTP(rec, req)

			if got := rec.Result().StatusCode; got != tt.wantStatus {
				t.Fatalf("status code = %d, want %d", got, tt.wantStatus)
			}
			if got := rec.Result().Header.Get("WWW-Authenticate"); got != tt.wantChallenge {
				t.Fatalf("WWW-Authenticate = %q, want %q", got, tt.wantChallenge)
			}
			if tt.wantBody != "" {
				var body struct {
					Error string `json:"error"`
				}
				if err := json.UnmarshalRead(rec.Result().Body, &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if !strings.Contains(rec.Body.String(), body.Error) || body.Error == "" {
					t.Fatalf("body = %q", rec.Body.String())
				}
			}
			if !tt.expectContext {
				return
			}
			if capturedCtx == nil {
				t.Fatalf("next handler context was not captured")
			}
			info, ok := capturedCtx.Value(userInfoKey).(userInfo)
			if !ok {
				t.Fatalf("context missing userInfo under userInfoKey")
			}
			if info.ID != "u-42" || info.Username != "alice" || info.Role != "admin" {
				t.Fatalf("userInfo = %+v", info)
			}
		})
	}
}

// testOAuthConfig is a minimal OAuth configuration; the guard only reads
// Issuer, Resource, and Scope from it.
func testOAuthConfig() *oauth.Config {
	return &oauth.Config{
		Issuer:   "https://karma.test/answer",
		Resource: "https://karma.test/answer/mcp",
		Scope:    "answer",
	}
}

func TestProtectMCPOAuthPair(t *testing.T) {
	t.Parallel()

	secret := "test-secret"
	oauthCfg := testOAuthConfig()
	guard := newAuthGuard(secret, oauthCfg)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := guard.protectMCP(next)

	signOAuthToken := func(t *testing.T, scope string) string {
		t.Helper()
		token, err := HS256Signer{Secret: secret, Issuer: oauthCfg.Issuer, Audience: oauthCfg.Resource}.Sign("alice", scope, time.Hour)
		if err != nil {
			t.Fatalf("sign oauth token: %v", err)
		}
		return token
	}

	t.Run("oauth_token_with_scope_passes", func(t *testing.T) {
		t.Parallel()
		var capturedCtx context.Context
		withCapture := guard.protectMCP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		}))
		req := bearerRequest(t, "Bearer "+signOAuthToken(t, "answer"))
		rec := httptest.NewRecorder()
		withCapture.ServeHTTP(rec, req)
		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d", rec.Result().StatusCode)
		}
		// OAuth tokens carry no user_id; the username doubles as the shard key.
		id, username := getUserInfo(capturedCtx)
		if id != "alice" || username != "alice" {
			t.Fatalf("getUserInfo = %q, %q", id, username)
		}
	})

	t.Run("oauth_token_without_scope_is_forbidden", func(t *testing.T) {
		t.Parallel()
		req := bearerRequest(t, "Bearer "+signOAuthToken(t, ""))
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		if rec.Result().StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d", rec.Result().StatusCode)
		}
		if got, want := rec.Result().Header.Get("WWW-Authenticate"), `Bearer realm="answer-mcp", error="insufficient_scope"`; !strings.HasPrefix(got, want) {
			t.Fatalf("WWW-Authenticate = %q", got)
		}
	})

	t.Run("challenge_advertises_resource_metadata", func(t *testing.T) {
		t.Parallel()
		req := bearerRequest(t, "")
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		if rec.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Result().StatusCode)
		}
		challenge := rec.Result().Header.Get("WWW-Authenticate")
		for _, want := range []string{
			`realm="answer-mcp"`,
			`error="invalid_token"`,
			`resource_metadata="https://karma.test/.well-known/oauth-protected-resource/answer/mcp"`,
			`scope="answer"`,
		} {
			if !strings.Contains(challenge, want) {
				t.Fatalf("challenge %q missing %q", challenge, want)
			}
		}
	})

	t.Run("static_token_still_passes_beside_oauth", func(t *testing.T) {
		t.Parallel()
		token, err := GenerateStaticToken(secret, "admin", time.Hour)
		if err != nil {
			t.Fatalf("GenerateStaticToken: %v", err)
		}
		req := bearerRequest(t, "Bearer "+token)
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d", rec.Result().StatusCode)
		}
	})
}

// TestUserInfoReachesToolContext proves the guard's identity survives mcp-go's
// StreamableHTTP context construction: the tool handler must see the actor
// that protectMCP authenticated.
func TestUserInfoReachesToolContext(t *testing.T) {
	t.Parallel()

	secret := "propagation-secret"
	var captured context.Context

	mcpServer := server.NewMCPServer("answer-test", "0", server.WithToolCapabilities(true))
	mcpServer.AddTool(mcp.NewTool("probe"), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		captured = ctx
		return mcp.NewToolResultText("ok"), nil
	})

	streamable := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithDisableLocalhostProtection(true),
	)
	srv := httptest.NewServer(newAuthGuard(secret, nil).protectMCP(streamable))
	t.Cleanup(srv.Close)

	token, err := GenerateStaticToken(secret, "alice", time.Hour)
	if err != nil {
		t.Fatalf("GenerateStaticToken: %v", err)
	}
	mcpClient, err := client.NewStreamableHttpClient(srv.URL+"/mcp",
		transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = mcpClient.Close() })

	ctx := t.Context()
	if err := mcpClient.Start(ctx); err != nil {
		t.Fatalf("start client: %v", err)
	}
	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_LEGACY_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "test", Version: "0"},
	}})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "probe"}}); err != nil {
		t.Fatalf("call tool: %v", err)
	}

	id, username := getUserInfo(captured)
	if id != "alice" || username != "alice" {
		t.Fatalf("getUserInfo in tool context = %q, %q; want alice, alice", id, username)
	}
}
