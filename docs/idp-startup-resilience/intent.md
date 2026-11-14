# Intent: gateway startup must not hang on the identity provider

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written from PR 5 findings at the owner's request without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

When the gateway starts, `RequireAuth` (`services/gateway/auth.go`) calls
`DumpRawJWKS`. That makes an `http.Get` to `http://<CIVIL_OS_GATEWAY_IDP_HOST>/keys`
using `http.DefaultClient`, which has **no timeout**. The call only logs the key
set at DEBUG level, yet it blocks startup:

- If the IdP (Dex or a jurisdiction's own OIDC server) accepts the connection and
  never answers, the gateway never starts serving. Kubernetes then restarts it in
  a loop, and nothing in the logs says why.
- PR 5 hit this in practice: a test pointing at an unused port hung for 30
  seconds behind a proxy.
- The fetch always uses `http://`, even when `CIVIL_OS_GATEWAY_AUTH_SERVER` uses
  `https://`.

A related auth-logging problem is still open: the auth middleware logs the raw
bearer token at DEBUG (`"Request contains token"`). Anyone with log access at
that level can replay the token.

Today this affects the owner, who operates every cluster. Later it affects anyone
running civil-os next to an IdP that is slow or still starting.

## Proposed outcome

1. **The gateway's startup time is bounded, whatever state the IdP is in.** No
   network call made during startup can block without limit.
2. **IdP trouble is visible.** An unreachable or slow IdP produces a clear log line
   naming the URL and the failure, and the health or readiness signal reflects it
   in whatever way the spec chooses.
3. **Token verification is also bounded.** Fetching keys while verifying a token
   cannot hold a request open longer than the request's own deadline.
4. **Logs never contain bearer tokens**, at any level.

## Constraints

- Keep the startup behaviour from PR 4: the gateway fails fast on missing or
  invalid auth configuration.
- No new required environment variables. A timeout may be configurable, but it
  needs a sensible default.
- Cluster configuration (Helm chart, gitops repo) is not edited from this repo.

## Success criteria

- A test with an IdP that accepts connections but never responds: the gateway's
  handler is ready within the bounded time, and the log names the problem.
- A test confirms no log record at any level contains the token string.
- Existing auth tests keep passing.

## Open questions (carry into spec)

- If the IdP is unreachable at startup, should the gateway start and report not
  ready, or exit? Not ready seems kinder to a slow-starting Dex, but exiting
  matches the fail-fast stance.
- Keep the JWKS diagnostic dump (bounded and optional) or remove it? go-oidc
  already fetches keys lazily.
- What timeout default? Is one setting enough for both startup and per-request key
  fetches?
- Should the diagnostic fetch follow the auth server's scheme (`https://`) instead
  of always using `http://`?
