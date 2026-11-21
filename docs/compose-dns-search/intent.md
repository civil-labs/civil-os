# Intent: compose service names never resolve outside the stack

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-20 |
| **Status** | Draft, written from a Record Keeper smoke run at the owner's request. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

In the local compose stack, services find each other by service name:
`database-reader:50051`, `database-writer:50052`, `tile-server:3000`, `dex:5556`,
`dex:5557` and `db:5432`. Docker's embedded DNS (`127.0.0.11`) answers those names
only while the container exists. Once a container is stopped, the name falls through
to the host's resolvers, **with the host's search domain appended**:

- Docker writes each container's `/etc/resolv.conf` when the container starts,
  copying the host's `search` line, with `ndots:0`. A name the embedded DNS can't
  answer is retried as `<name>.<host search domain>`.
- Some networks answer every name under their search domain. Public Wi-Fi often
  does, to redirect browsers to a sign-in page. There, a stopped service's name
  resolves to a **public IP address on the internet**.

A Record Keeper smoke run on 2026-11-19 hit this. With `database-reader` stopped,
the gateway resolved `database-reader` to a public IP and dialed it on port 50051,
and the request timed out after 30 seconds (`504 deadline_exceeded`, `dial tcp
<public ip>:50051: i/o timeout`). On a network without wildcard DNS, the lookup
fails at once and the client gets `unavailable`.

Two things make this worse than a slow error:

- **Credentials can leave the machine.** The gateway's tile proxy
  (`services/gateway/main.go`) forwards the whole request, including the user's
  `Authorization: Bearer` ID token, over plain HTTP to `tile-server:3000`. With the
  tile server stopped, a tile request would send the token to whoever answers at that
  IP. Likewise, with `db` stopped, the reader, writer and tile server would connect
  to whoever answers for `db:5432`, and a server there can ask a Postgres client for
  its password in cleartext. The local tokens and passwords are throwaway, but the pattern isn't
  one to keep.
- **It changes with the network, and it goes stale.** The search domain is copied
  when a container starts, so a long-running container keeps the one from wherever it
  started. In the same stack, the gateway (started the day before) and the reader
  (restarted that day, on another network) carried two different search domains. The stack behaves differently depending on where
  the laptop has been.

This affects anyone running the stack on a laptop: the owner, contributors, and
Record Keeper's development and smoke runs, which use this stack. Tests that stop a
service on purpose (the Record Keeper checklist's "a failed request" check, for
example) give network-dependent results.

## Proposed outcome

1. **A compose service name never resolves to anything outside the stack.** Whether a
   container is running, stopped or removed, a lookup of another service's name
   returns that container or fails. It never falls through to the host's search
   domain.
2. **A stopped service fails fast.** Calling a stopped service gives a prompt
   connection error, so the gateway returns `unavailable` within a second or two on
   any network.
3. **The stack behaves the same on every network.** Nothing in name resolution
   depends on the host's resolver configuration when a container started.
4. **Outside names still work** wherever the stack needs them, such as image pulls
   during builds.

## Constraints

- The local stack only: `compose.yaml` and `dev/`. Cluster configuration (Helm chart,
  gitops repo) is not edited from this repo.
- No change to the services' code or configuration names, unless the spec finds
  compose can't do it alone.
- Keep `mise run up` and `mise run test:e2e` working unchanged for a developer.
- No real hostnames, search domains or IP addresses in the repo. Tests use a
  synthetic domain.

## Success criteria

- With each of `database-reader`, `database-writer`, `tile-server`, `dex` and `db`
  stopped in turn, a lookup of its name from another container fails, and doesn't
  resolve to an outside address, even when the host has a search domain with
  wildcard DNS. A test simulates that host, or checks that no service's
  `/etc/resolv.conf` carries a search domain.
- With `database-reader` stopped, a public read through the gateway returns
  `unavailable` within 2 seconds.
- `mise run test:e2e` passes.

## Open questions (carry into spec)

- Which mechanism? Some candidates, none tested:
  - `dns_search` set so that no host search domain is inherited (whether `.` or an
    empty list does that in the Docker and Compose versions we support);
  - addressing services by rooted names (`database-reader.`), which resolvers don't
    extend with a search domain;
  - an `internal: true` network for service-to-service traffic, which also blocks
    egress, with only the gateway and Dex on a second network for published ports.
- Does the fix need to cover Podman as well as Docker, given the SELinux work
  (`docs/compose-selinux-relabel/`)?
- Should the gateway's tile proxy stop forwarding the `Authorization` header to the
  tile server, which doesn't need it? It's a separate fix, but this is where the risk
  showed up.
