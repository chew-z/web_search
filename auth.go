package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chew-z/web_search/internal/oauth"
	"github.com/golang-jwt/jwt/v5"
)

// staticIssuer, staticAudience, authRealm, and maxMCPBodyBytes define the
// static authentication values and the MCP request-body limit.
const (
	staticIssuer    = "answer-mcp"
	staticAudience  = "answer-mcp-user"
	authRealm       = "answer-mcp"
	maxMCPBodyBytes = 4 << 20
)

// actorKey identifies the authenticated username in a request context.
type actorKey struct{}

// Claims carries what the project's own tokens hold: static tokens minted by
// `mcp -generate-token` carry a username only; OAuth tokens carry the scope as
// well. Both sit over the registered claims.
type Claims struct {
	Username string `json:"username"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// SignToken signs a JWT for the supplied identity, audience, scope, and lifetime.
func SignToken(secret, issuer, audience, username, scope string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("token TTL must be greater than 0")
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Username: username,
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuer, Audience: jwt.ClaimStrings{audience}, IssuedAt: jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})
	value, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return value, nil
}

// GenerateStaticToken creates a legacy static bearer token with the project's
// fixed issuer and audience.
func GenerateStaticToken(secret, username string, ttl time.Duration) (string, error) {
	return SignToken(secret, staticIssuer, staticAudience, username, "", ttl)
}

// HS256Signer signs OAuth tokens with the configured HMAC secret and claims.
type HS256Signer struct{ Secret, Issuer, Audience string }

// _ verifies at compile time that HS256Signer implements oauth.Signer.
var _ oauth.Signer = HS256Signer{}

// Sign creates an OAuth token using the configured HS256 signer.
func (s HS256Signer) Sign(username, scope string, ttl time.Duration) (string, error) {
	return SignToken(s.Secret, s.Issuer, s.Audience, username, scope, ttl)
}

// extractTokenFromHeader extracts a Bearer token from an Authorization header.
// Uses strings.Fields for robust whitespace handling.
func extractTokenFromHeader(authHeader string) string {
	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// parseBearerToken validates an HTTP bearer token and returns its claims.
// The signing method is pinned to HS256 (prevents algorithm-confusion
// attacks), expiry is required, and clock skew gets 60 seconds of leeway.
func parseBearerToken(secret string, r *http.Request) (*Claims, error) {
	tokenString := extractTokenFromHeader(r.Header.Get("Authorization"))
	if tokenString == "" {
		return nil, fmt.Errorf("malformed bearer token")
	}
	parsedClaims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, parsedClaims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithLeeway(60*time.Second), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("parse bearer token: %w", err)
	}
	if strings.TrimSpace(parsedClaims.Username) == "" {
		return nil, fmt.Errorf("token username is empty")
	}
	return parsedClaims, nil
}

// tokenPair describes an accepted issuer, audience, and optional scope.
type tokenPair struct{ issuer, audience, scope string }

// authGuard holds the secret and token profiles used to protect the MCP endpoint.
type authGuard struct {
	secret string
	pairs  []tokenPair
	oauth  *oauth.Config
}

// newAuthGuard builds the static pair (answer-mcp/answer-mcp-user, no scope)
// and — when OAuth is configured — adds its issuer/resource pair, which
// requires its scope.
func newAuthGuard(secret string, oauthCfg *oauth.Config) *authGuard {
	guard := &authGuard{secret: secret, pairs: []tokenPair{{issuer: staticIssuer, audience: staticAudience}}, oauth: oauthCfg}
	if oauthCfg != nil {
		guard.pairs = append(guard.pairs, tokenPair{issuer: oauthCfg.Issuer, audience: oauthCfg.Resource, scope: oauthCfg.Scope})
	}
	return guard
}

// protectMCP enforces auth at the HTTP layer so clients receive a real 401 and
// WWW-Authenticate (claude.ai starts OAuth only on that); it caps bodies
// because mcp-go uses io.ReadAll, a nil guard means open mode, and OPTIONS
// passes CORS preflight. Tool handlers see the authenticated username through
// actorKey: mcp-go derives the tool context from the request context
// (streamable_http.go), so the value set here flows through.
func (g *authGuard) protectMCP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxMCPBodyBytes)
		if g == nil || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		parsedClaims, err := parseBearerToken(g.secret, r)
		if err != nil {
			Debug("auth rejected", "reason", err.Error())
			g.writeChallenge(w, "invalid_token")
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		for _, pair := range g.pairs {
			if parsedClaims.Issuer != pair.issuer || !hasAudience(parsedClaims.Audience, pair.audience) {
				continue
			}
			if pair.scope != "" && parsedClaims.Scope != pair.scope {
				Debug("auth rejected", "reason", "insufficient_scope", "username", parsedClaims.Username)
				g.writeChallenge(w, "insufficient_scope")
				writeAuthError(w, http.StatusForbidden, "insufficient_scope")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, parsedClaims.Username)))
			return
		}
		Debug("auth rejected", "reason", "unknown issuer/audience", "username", parsedClaims.Username)
		g.writeChallenge(w, "invalid_token")
		writeAuthError(w, http.StatusUnauthorized, "unauthorized")
	})
}

// hasAudience reports whether the expected audience is present in a JWT claim set.
func hasAudience(audiences jwt.ClaimStrings, audience string) bool {
	for _, candidate := range audiences {
		if candidate == audience {
			return true
		}
	}
	return false
}

// writeChallenge writes a Bearer challenge, adding resource_metadata only when
// OAuth is enabled for RFC 9728 discovery.
func (g *authGuard) writeChallenge(w http.ResponseWriter, code string) {
	challenge := `Bearer realm="` + authRealm + `", error="` + code + `"`
	if g.oauth != nil {
		challenge += `, resource_metadata="` + oauth.IssuerOrigin(g.oauth.Issuer) + oauth.ProtectedResourceMetadataPath(g.oauth.Resource) + `", scope="` + g.oauth.Scope + `"`
	}
	w.Header().Set("WWW-Authenticate", challenge)
}

// writeAuthError writes a JSON authentication error with the supplied HTTP status.
func writeAuthError(w http.ResponseWriter, status int, code string) {
	respondJSON(w, status, map[string]string{"error": code})
}

// actorFrom returns the authenticated username stored in a context.
// It returns false when the context carries no identity (e.g. stdio or open mode).
func actorFrom(ctx context.Context) (string, bool) {
	actor, ok := ctx.Value(actorKey{}).(string)
	return actor, ok
}
