---
title: Answer operations
date: 2026-10-21
status: active
tags: [operations, deployment, oauth]
---

# Answer operations

> Build, deploy, verify, and connect clients to the Answer MCP server.
> For the HTTP authentication design, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Configuration

The server reads its provider API key and the auth/OAuth variables below;
`MODEL`, `EFFORT`, `TIMEOUT`, `QUESTION`, `SHOW_ALL`, and `PROMPT_CACHE_KEY`
are CLI-only — MCP callers pass the equivalent tool parameters.
Loading lives in `config.go:126-151` (strict env parsing), `config.go:262-283`
(`loadOAuthEnv`), and `main.go:76-95` (the one-shot credential commands).

| Variable | Default | Meaning |
| --- | --- | --- |
| `PROVIDER` | `openai` | Backend selector: `openai` or `deepseek`; the `-provider` flag wins. |
| `OPENAI_API_KEY` | none; required for `openai` | OpenAI Responses API key. |
| `DEEPSEEK_API_KEY` | none; required for `deepseek` | DeepSeek key. |
| `GEMINI_AUTH_SECRET_KEY` | none | HS256 secret shared with GeminiMCP for token interoperability; required with `-auth-enabled`, ≥ 32 bytes with OAuth. |
| `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` | `true` | Keeps mcp-go's DNS-rebinding check off; the proxy forwards the public `Host` header. |
| `ANSWER_MCP_OAUTH` | `false` | Enable the OAuth 2.1 server; implies auth is enabled (`config.go:245-249`, `config.go:262-283`). |
| `ANSWER_MCP_ISSUER` | none | Public OAuth issuer URL: `https://karma.rrj.pl/answer`. |
| `ANSWER_MCP_RESOURCE` | none | Public protected-resource URL: `https://karma.rrj.pl/answer/mcp`. |
| `ANSWER_MCP_LOGIN_USER` | none | Browser-login username. |
| `ANSWER_MCP_LOGIN_PASSWORD_HASH` | none | PBKDF2 hash from `mcp -hash-password`. |
| `ANSWER_MCP_ACCESS_TTL` | `1h` | OAuth access-token lifetime. |
| `ANSWER_MCP_REFRESH_TTL` | `720h` | OAuth refresh-token lifetime; must exceed the access TTL. |

A malformed value in any boolean or duration variable stops startup with the
variable name and the value (`config.go:126-151`).
With OAuth on, startup also validates the issuer and resource as absolute URLs,
the login user, the password-hash format, and the TTL ordering
(`config.go:288-316`, `internal/oauth/config.go`).

## Build

```sh
task build
```

The output is `bin/answer`.
Do not use `go install`: the module path `github.com/chew-z/web_search`
produces a `web_search` binary (see the naming gotcha in
[ARCHITECTURE.md](ARCHITECTURE.md#naming-gotcha)).

## VPS environment and systemd

Production runs `answer.service` on karma, executing
`bin/answer mcp -port 8086 -transport http` with the provider set in the unit's
`Environment=` block; deployment is a manual build-and-copy.
The unit on the box is the source of truth for paths; the OAuth additions to
its `Environment=` block are:

```ini
Environment=ANSWER_MCP_OAUTH=true
Environment=ANSWER_MCP_ISSUER=https://karma.rrj.pl/answer
Environment=ANSWER_MCP_RESOURCE=https://karma.rrj.pl/answer/mcp
Environment=ANSWER_MCP_LOGIN_USER=<oauth-login-user>
Environment=ANSWER_MCP_LOGIN_PASSWORD_HASH=<generated-password-hash>
Environment=GEMINI_AUTH_SECRET_KEY=<at-least-32-random-bytes>
```

Generate the login hash interactively, then place stdout in
`ANSWER_MCP_LOGIN_PASSWORD_HASH`:

```sh
bin/answer mcp -hash-password
```

The hash format is dot-separated (`pbkdf2-sha256.<iterations>.<salt>.<key>`)
precisely so a systemd `Environment=` line, a shell, and a `.env` parser all
pass it through unquoted (`internal/oauth/password.go`).

The program blank-imports `godotenv/autoload`, which never replaces an
already-set variable, so unit `Environment=` values take precedence over
`.env` (the precedence note in [ARCHITECTURE.md](ARCHITECTURE.md)).
After editing the unit:

```sh
sudo systemctl daemon-reload
sudo systemctl restart answer
sudo systemctl status answer --no-pager
```

## OAuth discovery paths

`ProtectedResourceMetadataPath` appends the resource path to
`/.well-known/oauth-protected-resource`; `authorizationServerMetadataPath`
appends the issuer path to `/.well-known/oauth-authorization-server`
(`internal/oauth/oauth_metadata.go:97-117`).
`Mount` registers both derived paths and their bare compatibility paths
(`internal/oauth/mount.go:9-31`).

For the production issuer and resource, the required paths are:

- `/.well-known/oauth-protected-resource/answer/mcp`
- `/.well-known/oauth-authorization-server/answer`

## nginx

Install the three blocks in
[deploy/nginx-answer.conf](../deploy/nginx-answer.conf) inside karma's TLS
server block, and in the same edit remove `answer` from the shared
`location ~ ^/(?:answer|gemini|time)/(.*)$` alternation and from
`map $uri $mcp_backend` if it is listed there.
That shared location denies GET, but OAuth browser authorization requires
`GET /authorize`, and the alternation forwards the unstripped URI, which the
in-process OAuth routes would never match.
The exact-match well-known locations preserve the discovery paths, and the
trailing slash on the service `proxy_pass` strips `/answer/` before forwarding.

Validate nginx before reloading it:

```sh
sudo nginx -t && sudo systemctl reload nginx
```

## Verification

With the production service and nginx configuration active, run these from a
trusted shell:

```sh
curl -si -X POST https://karma.rrj.pl/answer/mcp
curl -fsS -o /dev/null -w '%{http_code}\n' \
  https://karma.rrj.pl/.well-known/oauth-protected-resource/answer/mcp
curl -fsS -o /dev/null -w '%{http_code}\n' \
  https://karma.rrj.pl/.well-known/oauth-authorization-server/answer
curl -fsS -o /dev/null -w '%{http_code}\n' https://karma.rrj.pl/answer/health
curl -fsS -o /dev/null -w '%{http_code}\n' https://karma.rrj.pl/answer/capabilities
```

The first command must return HTTP 401 with `WWW-Authenticate` containing
`resource_metadata` (`auth.go:144-175`, `auth.go:200-206`).
The two discovery commands, health, and capabilities must return HTTP 200;
the mux registers health and capabilities outside the guard
(`transport.go:111-118`).
A request carrying an existing static JWT must still succeed — the guard
accepts the legacy `gemini-mcp`/`gemini-mcp-user` pair beside the OAuth pair
(`auth.go:130-138`).

## Connect from Claude

### Claude.ai, Desktop, and mobile

These clients use the connector flow.
Add a custom connector with URL `https://karma.rrj.pl/answer/mcp`.
Leave Client ID and Client Secret empty so the server dynamically registers
the client through `/register` (`internal/oauth/mount.go:9-31`).
Complete the browser login with the configured OAuth login user and password.

### Claude Code

Add a project `.mcp.json` entry without headers to start the OAuth login flow:

```json
{
  "mcpServers": {
    "answer": {
      "type": "http",
      "url": "https://karma.rrj.pl/answer/mcp"
    }
  }
}
```

Run `/mcp` in Claude Code to open the OAuth login.
For scripts or configurations that prefer a header, generate a static token:

```sh
bin/answer mcp -generate-token -token-username <user> -token-ttl <duration>
```

`-token-username` defaults to `admin`, and `-token-ttl` is a Go duration that
defaults to `744h` (`main.go:48-51`, `mcp_cli.go:39-53`).
It prints a static bearer token and requires only `GEMINI_AUTH_SECRET_KEY`.
Configure it as a header instead of using the OAuth-only entry:

```json
{
  "mcpServers": {
    "answer": {
      "type": "http",
      "url": "https://karma.rrj.pl/answer/mcp",
      "headers": {
        "Authorization": "Bearer <token from -generate-token>"
      }
    }
  }
}
```

For a Claude Code "Conflicting scopes" error, remove the stale local entry and
run `/mcp` again:

```sh
claude mcp remove answer -s local
```

## Troubleshooting

- A missing closing quote in a systemd `Environment=` line silently drops that variable.
- Run `systemctl daemon-reload` after every unit-file edit and before restart.
- OAuth configuration failures stop startup with an error that identifies the
  invalid or absent value (`config.go:288-316`).
- Leave `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` at its default `true` behind
  the supplied nginx proxy; mcp-go would otherwise reject the forwarded public
  `Host` header as a DNS-rebinding risk (`transport.go`).
- Every bearer now needs an `exp` claim (`auth.go:99-118`); tokens minted
  without one stop validating — regenerate them with `-generate-token`.
- OAuth state is in memory, so a restart logs connector sessions out once
  their access token expires and requires a new browser login
  (`transport.go:116`, `oauth.NewStore(nil)`).
- Access tokens are stateless JWTs: one already issued stays valid until its
  expiry even after a restart or a password change.
  Keep `ANSWER_MCP_ACCESS_TTL` short (the 1h default) to bound that window.
