# External Go node

This process runs on the machine that owns the Chrome profile, cookies and
Flow bridge. It is intentionally not part of the Gateway container.

Required environment:

```text
FLOW_NODE_ID=node-a
FLOW_NODE_ADVERTISE_URL=http://10.0.0.21:9223
FLOW_GATEWAY_URL=http://gateway.internal:8001
FLOW_GATEWAY_SECRET=<same value used by the Gateway registry API>
FLOW_INTERNAL_TOKEN=<token used by the Go node API>
FLOW_COOKIE_DIR=<node-local cookie directory>
FLOW_DATA_DIR=<node-local data directory>
FLOW_OUTPUT_DIR=<node-local output directory>
```

Start the service with the native binary:

```text
flow-windows.exe service --listen 0.0.0.0:9223 --workers 8
```

The node sends an outbound registration/heartbeat to the Gateway. The Gateway
must be able to reach `FLOW_NODE_ADVERTISE_URL` for job dispatch and media
streaming, normally through a private network or VPN. Never expose the bridge
port to the public Internet.

Set the Gateway's `FLOW_NODE_TOKEN` to the same value as
`FLOW_INTERNAL_TOKEN` on all nodes for the initial deployment. A later
hardening step can switch to per-node tokens via `FLOW_NODE_TOKENS`.

`FLOW_NODE_ID` must remain stable across restarts. A node should be drained in
the Gateway before it is stopped so new jobs are no longer assigned to it.
