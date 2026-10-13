package main

import (
	context "context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func TestValidateJWT(t *testing.T) {
	t.Parallel()

	now := time.Now()
	secret := []byte("test-secret")
	wrongSecret := []byte("wrong-secret")

	mustMakeToken := func(claims Claims, method *jwt.SigningMethodHMAC, signSecret []byte) string {
		t.Helper()
		token := jwt.NewWithClaims(method, claims)
		signed, err := token.SignedString(signSecret)
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}
		return signed
	}

	tests := []struct {
		name          string
		token         string
		wantErr       bool
		wantErrType   error
		validateErrFn func(error) bool
	}{
		{
			name: "valid_hs256_token",
			token: mustMakeToken(
				Claims{
					UserID:    "u-1",
					Username:  "alice",
					Role:      "user",
					Issuer:    "gemini-mcp",
					Audience:  jwt.ClaimStrings{"gemini-mcp-user"},
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				jwt.SigningMethodHS256,
				secret,
			),
			wantErr: false,
		},
		{
			name: "expired_token",
			token: mustMakeToken(
				Claims{
					UserID:    "u-1",
					Username:  "alice",
					Role:      "user",
					Issuer:    "gemini-mcp",
					Audience:  jwt.ClaimStrings{"gemini-mcp-user"},
					ExpiresAt: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
					IssuedAt:  jwt.NewNumericDate(now.Add(-3 * time.Minute)),
				},
				jwt.SigningMethodHS256,
				secret,
			),
			wantErr:     true,
			wantErrType: jwt.ErrTokenExpired,
		},
		{
			name: "wrong_issuer",
			token: mustMakeToken(
				Claims{
					UserID:    "u-1",
					Username:  "alice",
					Role:      "user",
					Issuer:    "not-gemini-mcp",
					Audience:  jwt.ClaimStrings{"gemini-mcp-user"},
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				jwt.SigningMethodHS256,
				secret,
			),
			wantErr: true,
		},
		{
			name: "wrong_audience",
			token: mustMakeToken(
				Claims{
					UserID:    "u-1",
					Username:  "alice",
					Role:      "user",
					Issuer:    "gemini-mcp",
					Audience:  jwt.ClaimStrings{"not-gemini-mcp-user"},
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				jwt.SigningMethodHS256,
				secret,
			),
			wantErr: true,
		},
		{
			name: "wrong_signing_method",
			token: mustMakeToken(
				Claims{
					UserID:    "u-1",
					Username:  "alice",
					Role:      "user",
					Issuer:    "gemini-mcp",
					Audience:  jwt.ClaimStrings{"gemini-mcp-user"},
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				jwt.SigningMethodHS384,
				secret,
			),
			wantErr: true,
		},
		{
			name:    "malformed_token_string",
			token:   "not-a-jwt-token",
			wantErr: true,
		},
		{
			name: "invalid_signature",
			token: mustMakeToken(
				Claims{
					UserID:    "u-1",
					Username:  "alice",
					Role:      "user",
					Issuer:    "gemini-mcp",
					Audience:  jwt.ClaimStrings{"gemini-mcp-user"},
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
				jwt.SigningMethodHS256,
				wrongSecret,
			),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			claims, err := validateJWT(secret, tt.token)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("validateJWT() unexpected error = %v", err)
				}
				if claims == nil || claims.UserID == "" {
					t.Fatalf("validateJWT() got nil claims")
				}
				return
			}
			if err == nil {
				t.Fatalf("validateJWT() expected error")
			}
			if tt.wantErrType != nil && !errors.Is(err, tt.wantErrType) {
				t.Fatalf("validateJWT() error = %v, want wrapped %v", err, tt.wantErrType)
			}
		})
	}
}

func TestNewAuthHTTPMiddleware(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret")
	now := time.Now()

	makeClaimsToken := func(issuer string, audience string, exp time.Time, method *jwt.SigningMethodHMAC) string {
		t.Helper()
		c := Claims{
			UserID:    "u-42",
			Username:  "alice",
			Role:      "admin",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		}
		token := jwt.NewWithClaims(method, c)
		tokenString, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return tokenString
	}

	tests := []struct {
		name          string
		setupHeader   func() string
		wantStatus    int
		wantDetail    string
		expectContext bool
	}{
		{
			name: "valid_token_reaches_handler_with_user_info",
			setupHeader: func() string {
				return fmt.Sprintf("Bearer %s", makeClaimsToken("gemini-mcp", "gemini-mcp-user", now.Add(time.Hour), jwt.SigningMethodHS256))
			},
			wantStatus:    http.StatusOK,
			expectContext: true,
		},
		{
			name: "missing_authorization_header",
			setupHeader: func() string {
				return ""
			},
			wantStatus: http.StatusUnauthorized,
			wantDetail: "missing_token",
		},
		{
			name: "malformed_token",
			setupHeader: func() string {
				return "Bearer not-a-jwt-token"
			},
			wantStatus: http.StatusUnauthorized,
			wantDetail: "malformed_token",
		},
		{
			name: "expired_token",
			setupHeader: func() string {
				return fmt.Sprintf("Bearer %s", makeClaimsToken("gemini-mcp", "gemini-mcp-user", now.Add(-2*time.Minute), jwt.SigningMethodHS256))
			},
			wantStatus: http.StatusUnauthorized,
			wantDetail: "expired_token",
		},
	}

	nextHandler := func() (http.Handler, *context.Context) {
		var capturedCtx context.Context
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		})
		return h, &capturedCtx
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			next, capturedCtxPtr := nextHandler()
			mw := newAuthHTTPMiddleware(secret, next)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com/mcp", http.NoBody)
			authHeader := tt.setupHeader()
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}

			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if got := rec.Result().StatusCode; got != tt.wantStatus {
				t.Fatalf("status code = %d, want %d", got, tt.wantStatus)
			}

			if tt.wantStatus != http.StatusOK {
				if got := rec.Result().Header.Get("WWW-Authenticate"); got != `Bearer realm="mcp"` {
					t.Fatalf("WWW-Authenticate = %q, want %q", got, `Bearer realm="mcp"`)
				}
				var body struct {
					Error  string `json:"error"`
					Detail string `json:"detail"`
				}
				if err := json.UnmarshalRead(rec.Result().Body, &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if body.Error == "" {
					t.Fatalf("missing error field")
				}
				if body.Detail != tt.wantDetail {
					t.Fatalf("detail = %q, want %q", body.Detail, tt.wantDetail)
				}
				return
			}

			if !tt.expectContext {
				return
			}
			capturedCtx := *capturedCtxPtr
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
