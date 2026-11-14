# Intent: let browser apps carry trace context through the gateway

| | |
|---|---|
| **Author / product owner** | Jackson |
| **Date** | 2026-11-14 |
| **Status** | Draft, written from PR 5 findings at the owner's request without a brainstorm. Accepted when merged. |
| **Repo** | `github.com/civil-labs/civil-os` |

## Problem

Monorepo decision C2 makes the database-writer record a `trace_id` on every edit.
It takes the ID from the W3C `traceparent` header, so a change in the history
tables can be traced back to the request that made it. The gateway forwards
`traceparent` to the writer on `UpdateParcel`.

Browser clients can't use this:

- The gateway's CORS middleware lists the request headers browsers may send, and
  `traceparent` and `tracestate` are not on the list.
- A browser app with tracing turned on (for example OpenTelemetry web
  instrumentation) adds `traceparent` to cross-origin calls. The preflight then
  rejects them, so **the request fails outright**, not merely untraced.
- The only workaround is turning tracing off in the frontend. The writer then
  records a random `trace_id` for every browser-originated edit, and the
  frontend's trace and the database history can't be linked.

The planned frontend apps are the main users. Anyone who audits edits through
`trace_id` is affected too.

## Proposed outcome

1. **Browser apps on another origin can send `traceparent` and `tracestate`**, and
   the calls succeed.
2. **An edit made from a browser is recorded with the browser's trace ID**, so a
   frontend trace leads straight to the history rows it created.
3. **Trace context is handled the same way on every route** that needs it, so
   future RPCs and services don't each have to remember to forward headers.

## Constraints

- No change to the writer's C2 rules: use `traceparent` when it is valid,
  otherwise generate a random ID.
- Do not weaken CORS beyond adding these headers. Whether to keep the `*` origin
  is a separate decision.
- The public API contract (protos) does not change.

## Success criteria

- A CORS test: a preflight that asks for `traceparent` and `tracestate` succeeds,
  and the real request goes through.
- An end-to-end test: an `UpdateParcel` sent with a `traceparent` from a
  browser-style cross-origin request writes that trace ID to
  `parcel_attributes_history` / `parcel_attributes`.

## Open questions (carry into spec)

- Forward trace context on every mesh call, through a shared Connect interceptor,
  or only on writes as today?
- Should the gateway send trace context back to the browser (`traceresponse`, or
  an exposed header), so the frontend can show a reference ID?
- While touching this middleware: should it stop logging at INFO on every
  request, and should the allowed origin stay `*`?
