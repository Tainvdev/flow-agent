# Multi-node deployment

The supported deployment is now:

```text
clients -> FastAPI Gateway -> PostgreSQL
                         -> Go node A -> Chrome/Flow
                         -> Go node B -> Chrome/Flow
```

The Gateway has no Go binary, browser, extension, cookie directory or shared
media directory. Each Go node owns those resources locally and registers with
the Gateway using `FLOW_GATEWAY_URL` and `FLOW_GATEWAY_SECRET`.

## Gateway

```powershell
cd deploy/gateway
Copy-Item .env.example .env
# Edit .env and set POSTGRES_PASSWORD, FLOW_GATEWAY_SECRET and FLOW_NODE_TOKEN.
docker compose up -d --build
```

The Gateway's PostgreSQL database is a new control plane. Old SQLite data is
not imported.

## Go node

Each node must have a stable identity and a URL reachable by the Gateway:

```text
FLOW_NODE_ID=node-a
FLOW_NODE_ADVERTISE_URL=http://10.0.0.21:9223
FLOW_GATEWAY_URL=http://gateway.internal:8001
FLOW_GATEWAY_SECRET=<gateway registry secret>
FLOW_INTERNAL_TOKEN=<node API secret>
```

Start the native binary on the node:

```text
flow-windows.exe service --listen 0.0.0.0:9223 --workers 8
```

The extension connects to the node's bridge port. It does not connect to the
Gateway. The Gateway routes an account and its job to the same node for the
whole lifecycle. An accepted job is never silently retried on another node.

## Routing rules

- Existing job IDs and idempotency keys reuse their stored node.
- Media references route to the node that owns the media.
- Automatic jobs use the least-loaded healthy node.
- A node with an expired heartbeat is excluded from new work.

The initial implementation uses the same `FLOW_NODE_TOKEN` for node API calls.
Use a private network/VPN and replace it with per-node `FLOW_NODE_TOKENS` before
exposing nodes outside a trusted network.

Gateway dispatch uses bounded failover up to
`FLOW_GATEWAY_MAX_DISPATCH_ATTEMPTS`. Operational failures, including
`dispatch_unknown` and Go-node `submission_unknown`, are retried on another
healthy node when one is available. This deliberately prioritizes returning a
result over exactly-once execution: if the previous node actually submitted
the work before losing its acknowledgement, failover can create a duplicate.
Validation and authentication failures are not retried.

## Drain a node

Drain before maintenance so new jobs stop routing there while existing jobs
continue to finish:

```bash
curl -X PATCH http://gateway.internal:8001/api/v1/nodes/node-a \
  -H "Authorization: Bearer $FLOW_GATEWAY_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"status":"draining"}'
```

Re-enable it with `{"status":"online"}` after the node is ready. Heartbeats
preserve the draining state; stale/offline nodes become online again when they
heartbeat, unless explicitly drained.
