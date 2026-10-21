package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	json "encoding/json/v2"

	"github.com/chew-z/web_search/internal/oauth"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// testSecret is the ≥32-byte signing secret shared by the OAuth tests.
const testSecret = "this-secret-is-longer-than-thirty-two-bytes"

// oauthVerifier is the PKCE verifier; its S256 challenge is computed per flow.
const oauthVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func oauthRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, target, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return req
}

func oauthDo(t *testing.T, httpClient *http.Client, req *http.Request) (int, http.Header, string) {
	t.Helper()
	response, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, string(body)
}

// oauthHiddenField extracts a hidden form input from the login page HTML.
func oauthHiddenField(t *testing.T, body, name string) string {
	t.Helper()
	marker := `name="` + name + `" value="`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing %s in %q", name, body)
	}
	value := body[start+len(marker):]
	end := strings.IndexByte(value, '"')
	if end < 0 {
		t.Fatalf("unterminated %s", name)
	}
	return value[:end]
}

// testOAuthMCPConfig builds a validated MCP config with OAuth enabled and a
// real PBKDF2 hash of "pw" for the login user "tester".
func testOAuthMCPConfig(t *testing.T) MCPConfig {
	t.Helper()
	hash, err := oauth.HashPassword("pw")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	cfg := parseMCPConfig(MCPConfig{
		APIKey:                 "test-key",
		Transport:              "http",
		AuthEnabled:            true,
		AuthSecretKey:          testSecret,
		OAuthEnabled:           true,
		OAuthLoginUser:         "tester",
		OAuthLoginPasswordHash: hash,
		OAuthAccessTTL:         time.Hour,
		OAuthRefreshTTL:        2 * time.Hour,
	})
	return cfg
}

// oauthServer starts an httptest server wired exactly like production
// (buildHTTPHandler) with the issuer/resource pointed at its own URL.
func oauthServer(t *testing.T) (*httptest.Server, MCPConfig) {
	t.Helper()
	var handler http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}))
	cfg := testOAuthMCPConfig(t)
	cfg.OAuthIssuer = srv.URL
	cfg.OAuthResource = srv.URL + "/mcp"
	built, err := buildHTTPHandler(NewMCPServer(cfg), cfg)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	handler = built
	t.Cleanup(srv.Close)
	return srv, cfg
}

func oauthRegister(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	body := `{"client_name":"test client","redirect_uris":["https://example.test/callback"],"token_endpoint_auth_method":"none"}`
	req := oauthRequest(t, http.MethodPost, srv.URL+"/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	status, _, response := oauthDo(t, http.DefaultClient, req)
	if status != http.StatusCreated {
		t.Fatalf("register = %d: %s", status, response)
	}
	var registered struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal([]byte(response), &registered); err != nil || registered.ClientID == "" {
		t.Fatalf("registration = %s: %v", response, err)
	}
	return registered.ClientID
}

// obtainOAuthTokens walks the full browser flow: authorize (login form) →
// login → authorization code → token exchange.
func obtainOAuthTokens(t *testing.T, srv *httptest.Server, cfg MCPConfig, clientID string) oauth.TokenResponse {
	t.Helper()
	sum := sha256.Sum256([]byte(oauthVerifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {"https://example.test/callback"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {"state"},
	}
	httpClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	status, _, body := oauthDo(t, httpClient, oauthRequest(t, http.MethodGet, srv.URL+"/authorize?"+query.Encode(), nil))
	if status != http.StatusOK {
		t.Fatalf("authorize = %d: %s", status, body)
	}
	form := url.Values{
		"pending_id": {oauthHiddenField(t, body, "pending_id")},
		"csrf_token": {oauthHiddenField(t, body, "csrf_token")},
		"username":   {cfg.OAuthLoginUser},
		"password":   {"pw"},
	}
	login := oauthRequest(t, http.MethodPost, srv.URL+"/login", strings.NewReader(form.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status, header, body := oauthDo(t, httpClient, login)
	if status != http.StatusFound {
		t.Fatalf("login = %d: %s", status, body)
	}
	redirect, err := url.Parse(header.Get("Location"))
	if err != nil || redirect.Query().Get("code") == "" {
		t.Fatalf("redirect = %q, %v", header.Get("Location"), err)
	}
	form = url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {redirect.Query().Get("code")},
		"redirect_uri":  {"https://example.test/callback"},
		"client_id":     {clientID},
		"code_verifier": {oauthVerifier},
	}
	token := oauthRequest(t, http.MethodPost, srv.URL+"/token", strings.NewReader(form.Encode()))
	token.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status, _, body = oauthDo(t, httpClient, token)
	if status != http.StatusOK {
		t.Fatalf("token = %d: %s", status, body)
	}
	var tokens oauth.TokenResponse
	if err := json.Unmarshal([]byte(body), &tokens); err != nil || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("token response = %s: %v", body, err)
	}
	return tokens
}

// TestEndToEndOAuthThenTools walks discovery → registration → browser login →
// tokens → authenticated MCP use → refresh → static-JWT fallback → challenge.
func TestEndToEndOAuthThenTools(t *testing.T) {
	srv, cfg := oauthServer(t)
	clientID := oauthRegister(t, srv)
	tokens := obtainOAuthTokens(t, srv, cfg, clientID)
	listToolsWithBearer(t, srv.URL+"/mcp", tokens.AccessToken)

	refresh := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokens.RefreshToken},
		"client_id":     {clientID},
	}
	req := oauthRequest(t, http.MethodPost, srv.URL+"/token", strings.NewReader(refresh.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status, _, body := oauthDo(t, http.DefaultClient, req)
	if status != http.StatusOK {
		t.Fatalf("refresh = %d: %s", status, body)
	}
	var renewed oauth.TokenResponse
	if err := json.Unmarshal([]byte(body), &renewed); err != nil || renewed.AccessToken == "" {
		t.Fatalf("refresh response = %s: %v", body, err)
	}
	listToolsWithBearer(t, srv.URL+"/mcp", renewed.AccessToken)

	staticToken, err := GenerateStaticToken(testSecret, "admin", cfg.OAuthAccessTTL)
	if err != nil {
		t.Fatal(err)
	}
	listToolsWithBearer(t, srv.URL+"/mcp", staticToken)

	status, header, _ := oauthDo(t, http.DefaultClient, oauthRequest(t, http.MethodPost, srv.URL+"/mcp", nil))
	if status != http.StatusUnauthorized || !strings.Contains(header.Get("WWW-Authenticate"), "resource_metadata") {
		t.Fatalf("missing token = %d, %q", status, header.Get("WWW-Authenticate"))
	}
}

// TestOAuthDiscoveryDocuments asserts both well-known documents serve on the
// production wiring, at the derived sub-path locations and the bare
// compatibility paths, for a karma-shaped issuer/resource pair.
func TestOAuthDiscoveryDocuments(t *testing.T) {
	t.Parallel()
	cfg := testOAuthMCPConfig(t)
	cfg.OAuthIssuer = "http://host/answer"
	cfg.OAuthResource = "http://host/answer/mcp"
	handler, err := buildHTTPHandler(NewMCPServer(cfg), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{
		"/.well-known/oauth-protected-resource/answer/mcp",
		"/.well-known/oauth-authorization-server/answer",
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-authorization-server",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", target, response.Code, response.Body.String())
		}
	}
}

func listToolsWithBearer(t *testing.T, endpoint, bearer string) {
	t.Helper()
	mcpClient, err := client.NewStreamableHttpClient(endpoint, transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + bearer}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mcpClient.Close() })
	if err := mcpClient.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := mcpClient.Initialize(context.Background(), mcpgo.InitializeRequest{Params: mcpgo.InitializeParams{ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION, ClientInfo: mcpgo.Implementation{Name: "test", Version: "0"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := mcpClient.ListTools(context.Background(), mcpgo.ListToolsRequest{}); err != nil {
		t.Fatal(err)
	}
}
