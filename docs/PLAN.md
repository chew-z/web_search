---
title: OAuth 2.1 connector auth port
date: 2026-10-21
status: active
tags: [plan, oauth, mcp, deployment]
---

# Port OAuth 2.1 connector auth from the sibling repos

> Working plan for bringing Answer to the sibling standard:
> static-JWT HTTP auth **plus** an OAuth 2.1 authorization server,
> so claude.ai (Claude Desktop) connectors can log in through a browser
> while plain HTTP clients keep using a bearer token.
> Retire this file to `docs/retired/` when the work is consumed.

## Where the standard comes from

| Step | Repo | Commit | What it did |
| --- | --- | --- | --- |
| Origin | PolCritique | — | First OAuth 2.1 authorization server. |
| Port 1 | Alerts | `6cb21d9` | Server ported from PolCritique; JWT access tokens, `/mcp` middleware, `-hash-password`. |
| Port 2 | Pushover | `11331` | Package copied from Alerts with pushover defaults. |
| Dual mode | Pushover | `1ea1849` | OAuth at the HTTP layer **beside** the static JWT; the guard model Answer adopts. |
| Hardening | Pushover | `e731dec` | Rate-limiter and client-secret tests, login-CPU bound. |
| Port back | Alerts | `bb1e168` | `e731dec` ported into the Alerts copy; deployed to karma (`bb45c65`). |

The `internal/oauth` packages in Pushover and Alerts are now identical
(3,840 lines each, stdlib-only) except scope/realm defaults and consent-page wording.

**Pushover is the template for this port.**
It shares Answer's flat layout (`auth.go`, `config.go`, `http_server.go`, `mcp_cli.go`),
runs on karma behind nginx, and documents the whole deployment in
`docs/OPERATIONS.md` and `deploy/nginx-pushover.conf`.

### What the pattern consists of

1. **`internal/oauth`** — self-contained OAuth 2.1 authorization server:
   RFC 9728 protected-resource metadata, RFC 8414 authorization-server metadata
   (path-inserted well-known URLs for sub-path issuers),
   RFC 7591 dynamic client registration plus CIMD,
   authorization-code flow with PKCE S256, refresh-token families,
   a browser login page verified against a PBKDF2-SHA256 hash
   (600k iterations, shell-safe dotted format),
   a rate limiter (burst 10, refill 6 s), ceilings on every in-memory map,
   and a four-slot bound on concurrent password verification.
2. **`authGuard`** (`Pushover/auth.go`) — one HTTP-layer middleware validating
   HS256 bearers against a list of token pairs:
   the legacy static pair (fixed issuer/audience, no scope)
   plus the OAuth pair (issuer = OAuthIssuer, audience = OAuthResource, scope required).
   Failures return a real 401 with
   `WWW-Authenticate: Bearer ... resource_metadata="..."` —
   that header is what makes claude.ai start the OAuth flow.
   Also: 4 MiB body cap, OPTIONS pass-through, 403 on `insufficient_scope`.
3. **Config** — `*_MCP_OAUTH`, `_ISSUER`, `_RESOURCE`, `_LOGIN_USER`,
   `_LOGIN_PASSWORD_HASH`, `_ACCESS_TTL` (1 h), `_REFRESH_TTL` (720 h);
   OAuth implies auth enabled; secret at least 32 bytes; validated at startup.
4. **CLI helpers** — `mcp -generate-token` (static JWT) and
   `mcp -hash-password` (silent terminal read via `golang.org/x/term`).
5. **Wiring** — `oauth.Mount(mux, cfg, oauth.NewStore(nil), logger)` beside the
   guarded MCP path; the store is in-memory, so a restart logs OAuth sessions out.
6. **Deployment** — three nginx blocks (two exact well-known locations plus a
   prefix-stripping service location), and removal of the service from karma's
   shared GET-denying alternation.

## Decisions (settled)

| Question | Decision | Note |
| --- | --- | --- |
| OAuth scope | `answer` | Sibling convention is scope = service name (`pushover`, `alerts`). |
| Realm | `answer-mcp` | Mirrors `pushover-mcp`. |
| Static JWT issuer/audience | keep `gemini-mcp` / `gemini-mcp-user` | Existing tokens and GeminiMCP interoperability keep working. |
| Signing secret env var | keep `GEMINI_AUTH_SECRET_KEY` | Must be ≥ 32 bytes once OAuth is on. |
| New env prefix | `ANSWER_MCP_*` | Consistent with `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION`. |
| mcp-go | upgrade v0.56.0 → v1.3.0 as Phase 1 | Matches Pushover; sibling floor is v1.0.0 (PolCritique). |
| Token expiry | `exp` becomes required on all bearers | Pushover guard uses `jwt.WithExpirationRequired()`. |
| OAuth store | in-memory, restart = re-login | Accepted trade-off, same as both siblings. |
| nginx | prefix-stripping `proxy_pass` to `127.0.0.1:8086/` | The `/` catch-all MCP mount keeps working; OAuth routes match exactly. |
| HTTP timeouts | keep Answer's (15 m write) | Long web searches; nginx `proxy_read_timeout 24h` covers it. |
| Discovery endpoints | add `/health` and `/capabilities` | Parity with Pushover; both unauthenticated by design (monitoring probe, tool discovery). |

## Phases

### Phase 1 — upgrade mcp-go v0.56.0 → v1.3.0

Do this first and alone, so later breakage is unambiguous.

- [x] `go get github.com/mark3labs/mcp-go@v1.3.0`, `go mod tidy`.
- [x] Fix any compile fallout; Pushover proves the tool-handler signature
      `func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)`
      and the `StreamableHTTPOption` set are unchanged.
- [x] Skim the v1.0.0 release notes for behavior changes
      (session handling, validation, DNS-rebinding defaults).
- [x] Gate: `task check` green; smoke both transports locally
      (stdio handshake; HTTP `initialize` with a static JWT).

### Phase 2 — vendor `internal/oauth`

- [x] Copy all 14 files from `Pushover/internal/oauth/` (tests included)
      to `Answer/internal/oauth/`; import path `github.com/chew-z/web_search/internal/oauth`.
- [x] Full rename following the Alerts precedent (their copy has zero pushover
      references): `pushover` → `answer`, `pushover-mcp` → `answer-mcp` throughout
      including test fixtures; login page re-branded (`answer` / `web search answers`,
      consent copy `asking to search the web as you`).
      Defaults: `Scope` = `answer`, `Realm` = `answer-mcp`.
- [x] Gate: `go test ./internal/oauth/...` green (brings the 429, client-secret,
      login-bound and store race tests with it).

### Phase 3 — rework `auth.go` to the guard model

- [x] Add `SignToken`, `GenerateStaticToken`, and `HS256Signer` (implements `oauth.Signer`),
      with `staticIssuer = "gemini-mcp"`, `staticAudience = "gemini-mcp-user"`.
- [x] Merge the claims structs: `UserID`, `Username`, `Role` (legacy static tokens)
      plus `Scope` (OAuth tokens) over `jwt.RegisteredClaims`;
      `getUserInfo` (consumed at `api.go:216`) keeps working.
- [x] Replace `newAuthHTTPMiddleware` with `authGuard.protectMCP`:
      token-pair matching, `writeChallenge` with `resource_metadata` (RFC 9728 discovery),
      4 MiB `http.MaxBytesReader`, OPTIONS pass-through, 403 on `insufficient_scope`.
- [x] `copyActor` + `server.WithHTTPContextFunc` turned out to be unnecessary:
      mcp-go v1.3.0 derives the tool context from the request context
      (streamable_http.go:720), so the guard's `userInfo` value flows through as-is.
      Proven by `TestUserInfoReachesToolContext` (real StreamableHTTP round trip).
      OAuth tokens carry no `user_id`, so the username doubles as the cache-shard key.
- [x] Behavior changes accepted: `exp` required; 401 body becomes `{"error":"unauthorized"}`
      with the detail moving into the `WWW-Authenticate` error code; empty username rejected.

### Phase 4 — config, env, and CLI helpers

- [x] Extend `MCPConfig` (`config.go`) with
      `OAuthEnabled`, `OAuthIssuer`, `OAuthResource`, `OAuthLoginUser`,
      `OAuthLoginPasswordHash`, `OAuthAccessTTL` (1 h), `OAuthRefreshTTL` (720 h).
      `MCPConfigParams` collapsed into `MCPConfig` in the same change — the
      OPEN_ITEMS debt note named exactly this trigger; the item is removed.
- [x] Read `ANSWER_MCP_OAUTH`, `ANSWER_MCP_ISSUER`, `ANSWER_MCP_RESOURCE`,
      `ANSWER_MCP_LOGIN_USER`, `ANSWER_MCP_LOGIN_PASSWORD_HASH`,
      `ANSWER_MCP_ACCESS_TTL`, `ANSWER_MCP_REFRESH_TTL` (`loadOAuthEnv`);
      `parseEnvBool` now rejects unparseable values instead of silently defaulting.
- [x] `Validate()`: OAuth implies auth enabled (forced in `parseMCPConfig`);
      secret ≥ 32 bytes when OAuth is on; `OAuthConfig()` derives `oauth.Config`
      with the `HS256Signer` and validates at startup. `requireAuthSecret` folded in.
- [x] `mcp` subcommand (`main.go` + new `mcp_cli.go`): `-generate-token`,
      `-token-username`, `-token-ttl` (default 744 h), `-hash-password`;
      `readPassword` test seam ported; `golang.org/x/term` added. The one-shot
      helpers run before provider/API-key loading.
- [x] Smoked: CLI-minted token passes the guard (200); short secret and a bad
      `ANSWER_MCP_OAUTH` value fail startup with clear errors.

### Phase 5 — transport wiring

- [x] `RunHTTPTransport` (`transport.go`): build the guard when `AuthEnabled`,
      wrap the MCP handler, and `oauth.Mount(mux, cfg, oauth.NewStore(nil), SLogger())`
      when OAuth is enabled.
- [x] Keep the `/` catch-all for MCP (mcp-go routes by method, not path);
      Go's ServeMux gives the exact OAuth routes precedence.
- [x] Add `/health` (GET, JSON status) mirroring Pushover's `handleHealth`.
- [x] Add `/capabilities` (GET, JSON: name, version, description, transport, tools,
      authentication) mirroring Pushover's `handleCapabilities` + `capabilityTools`;
      the deterministic tool listing uses mcp-go v1 `ListTools()`, available after Phase 1.
- [x] Smoked live: metadata documents, `/health`, `/capabilities` (lists `gpt_websearch`),
      `/register` → 201, and `POST /` without a token → 401 with
      `resource_metadata` in the challenge. The smoke caught an ordering bug —
      `parseMCPConfig` forced `AuthEnabled` before `loadOAuthEnv` read
      `ANSWER_MCP_OAUTH`, leaving the guard nil; the implication is now enforced
      in `loadOAuthEnv` (config.go). JSON helper named `respondJSON` because
      `writeJSON` is taken by an api_test.go helper.

### Phase 6 — tests

- [x] Port `Pushover/oauth_e2e_test.go`: discovery → register → authorize → login →
      token → refresh → authenticated MCP call, plus the 401-challenge assertions.
      Runs against `buildHTTPHandler` (extracted from `RunHTTPTransport` in Phase 5
      so tests drive the production wiring); `TestOAuthDiscoveryDocuments` covers
      the karma-shaped derived well-known paths and the bare compatibility paths.
- [x] Guard cases in `auth_test.go`: static pair still accepted, OAuth pair accepted,
      wrong scope 403, expired/absent `exp` rejected, empty username rejected,
      OPTIONS pass-through, challenge carries `resource_metadata`, and
      `TestUserInfoReachesToolContext` proves identity propagation over a real
      StreamableHTTP round trip (landed with Phase 3).
- [x] Config cases in `config_test.go`: `loadOAuthEnv` defaults/forcing/errors,
      `Validate()` rules table, `OAuthConfig()` derivation (scope `answer`,
      realm `answer-mcp`, HS256Signer fields). CLI cases in `mcp_cli_test.go`:
      token generation round-trips through the guard parse, hash format, prompts,
      and error paths.
- [x] Gate: `task check` green; `go test -race ./...` green. Mutation checks:
      disabling the scope comparison fails `TestProtectMCPOAuthPair`, dropping
      `resource_metadata` from the challenge fails the challenge assertion.

### Phase 7 — deployment artifacts and docs

- [x] `deploy/nginx-answer.conf` modeled on `deploy/nginx-pushover.conf`:
      exact locations for `/.well-known/oauth-protected-resource/answer/mcp` and
      `/.well-known/oauth-authorization-server/answer`,
      plus `^~ /answer/` stripping the prefix to `http://127.0.0.1:8086/`,
      GET allowed, `client_max_body_size 4m`, buffering off, long read/send timeouts,
      `X-Forwarded-For` overwritten with `$remote_addr`.
- [x] systemd unit `Environment=` additions documented in OPERATIONS.md
      (the unit on the box stays the source of truth for paths):
      `ANSWER_MCP_OAUTH=true`, `ANSWER_MCP_ISSUER=https://karma.rrj.pl/answer`,
      `ANSWER_MCP_RESOURCE=https://karma.rrj.pl/answer/mcp`, login user + hash from
      `-hash-password`, TTLs at defaults.
- [x] `docs/OPERATIONS.md` (new, mirroring Pushover's): env reference, discovery paths,
      post-deploy verification curls, and Connect-from-Claude for all three client shapes —
      claude.ai custom connector (empty client ID/secret, dynamic registration, browser login),
      header-less `.mcp.json` entry (OAuth login via `/mcp` in Claude Code),
      and the existing static-JWT header entry.
- [x] `docs/ARCHITECTURE.md` rewritten for the guard model (components, MCP data flow,
      transport/auth, env table, flags, dependencies); `docs/STATUS.md` health and a
      2026-10-21 entry; `docs/OPEN_ITEMS.md` — README-endpoints defect resolved and
      removed, `Claims.Role` citations refreshed, in-memory-store and pending-deploy debt
      added; README endpoints/structure corrected; `AGENTS.md` / `CLAUDE.md` map rows for
      OPERATIONS.
- [x] Gate: `task lint:md` green.

### Phase 8 — karma deployment and live verification

The VPS builds natively from its checkout: push `main`, then on karma
`git pull` in `/home/debian/Go/src/Answer`, `task deps`, `task build`,
restart `answer.service`. Nothing here automates it.
A full local dress rehearsal passed against the built binary on 2026-10-21:
register → branded login page → code → token → authenticated MCP initialize
→ refresh → wrong-password rejection, all via curl on karma-shaped paths.
The edited `answer.service` and `karma.rrj.pl.conf` are staged in `tmp/`
(gitignored; they carry live credentials — handle with care).

- [x] Push `main`; on karma pull, `task deps`, `task build` (owner, 2026-10-21).
- [x] Install the edited unit (`tmp/answer.service` → `/etc/systemd/system/`),
      `daemon-reload`, restart `answer.service` (owner, 2026-10-21).
- [x] Edit karma's nginx **atomically**: three blocks added, `answer` removed from the
      shared `location ~ ^/(?:answer|gemini|time)/(.*)$` alternation and the
      `$mcp_backend` map in the same change; `nginx -t` and reload (owner, 2026-10-21).
- [x] Verified live from the dev box on 2026-10-21:
      `POST /answer/mcp` without a token → 401 with the full `resource_metadata`
      challenge; both well-known documents → 200 with correct contents;
      `/answer/health` and `/answer/capabilities` → 200; bare `GET /answer/authorize`
      → 400 from the OAuth server (edge passes GET); full curl flow
      register → branded login page → login → code → token → MCP initialize 200 →
      refresh; static JWT signed with the production secret (`gemini-mcp` pair, with
      `exp`) → 200; bogus token → 401.
- [ ] Add the claude.ai custom connector with URL `https://karma.rrj.pl/answer/mcp`,
      empty client ID/secret; complete the browser login and call `gpt_websearch`.
      (Owner's account and browser; optionally also the Claude Code `/mcp` login.)
- [ ] Record the connector login in `docs/STATUS.md`; retire this plan to `docs/retired/`.

## Risks and notes

- A service restart invalidates refresh tokens and registered clients (in-memory store);
  connectors eventually need a fresh browser login. Both siblings accepted this.
- The nginx alternation edit and the new blocks must land together,
  or existing `.mcp.json` JWT clients break mid-window.
- Static tokens minted without `exp` stop validating; regenerate via `-generate-token`.
- Keep the phases as separate commits; do not bundle unrelated refactors into this port.

## References

- `Pushover/auth.go`, `Pushover/config.go`, `Pushover/http_server.go`,
  `Pushover/mcp_cli.go`, `Pushover/internal/oauth/`, `Pushover/oauth_e2e_test.go`
- `Pushover/docs/OPERATIONS.md` — deployment runbook and Connect-from-Claude
- `Pushover/deploy/nginx-pushover.conf` — the three-block nginx pattern
- `Alerts/internal/oauth/` — identical package, consent wording differs
- `Answer/auth.go`, `Answer/transport.go`, `Answer/config.go`, `Answer/main.go` — current state
