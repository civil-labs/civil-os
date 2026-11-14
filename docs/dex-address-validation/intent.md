# Intent: catch a misconfigured Dex address at startup

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written from PR 5 findings at the owner's request without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

Monorepo decision C4 says the gateway serves the Dex admin API only when
`CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS` is set, and that **an invalid address is a
startup error**. The code relies on `grpc.NewClient` to reject bad addresses. PR 5
showed that it rejects almost nothing:

- `http://dex:5557`, `dex:5557`, `dns:///`, `unix:` and the empty string are all
  accepted.
- Only malformed URL escapes such as `%zz` fail.

So the likeliest mistakes pass the startup check: a URL with a scheme instead of
`host:port`, a typo, or a wrong port. They only show up when someone first calls
the admin API, as `Unavailable` on every call. That looks like a Dex outage, not a
configuration error. The gRPC authority is also hard-coded to `idp`, which nothing
documents.

Today this affects the owner, who configures each jurisdiction's gateway. Later
it affects any operator who enables the Dex admin API.

## Proposed outcome

1. **Configuration errors are reported at startup**, not at first use. If
   `CIVIL_OS_GATEWAY_DEX_GRPC_ADDRESS` is not a valid gRPC target, startup fails
   with a message that names the variable, the value and the expected form. The
   expected form is `host:port`, or an explicit gRPC resolver scheme such as
   `dns:///host:port`.
2. **"Dex is down" and "Dex is misconfigured" look different** in the logs and in
   the API's error codes.
3. **The behaviour is documented**: accepted address forms, what happens when Dex
   is unreachable at startup, and what the `idp` authority is for.

## Constraints

- Keep C4: when the variable is unset, the Dex admin API is not registered
  (`Unimplemented`). When Dex is unreachable at call time, calls return
  `Unavailable`.
- No new required environment variables.
- Cluster configuration (Helm chart, gitops repo) is not edited from this repo.

## Success criteria

- Tests show that `http://dex:5557`, an empty host, a missing port and an
  out-of-range port stop the gateway at startup with a helpful message.
- Tests show that `dex:5557` and `dns:///dex:5557` are accepted.
- The existing Dex tests (enabled, disabled, unreachable) keep passing.

## Open questions (carry into spec)

- Should startup also check that Dex is reachable (a gRPC health or `GetVersion`
  call)? If so, should an unreachable Dex fail startup, log a warning, or show in
  readiness? Dex may start after the gateway.
- Which resolver schemes count as valid: `dns`, `passthrough`, `unix`?
- Should the `idp` authority become configurable, or be documented as a
  requirement of the deployment's Envoy routing?
