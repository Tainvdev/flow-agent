## Goal

Keep existing Go generation request builders and response parsers, but route image and video generation RPC submissions through the Flow page tab attached to the matching browser profile. The extension performs the same-origin POST so Chrome supplies the page session; Go receives status and response text and continues existing retry and parsing logic.

## Scope

- Route generation submission RPCs: image generation, text-to-video, frame-based video, reference-image video, and video edit.
- Keep payload construction, envelope encoding, RPC query construction, anti-CSRF handling, retry/captcha refresh, and frame parsing in the Go batchexecute client.
- Keep reads, uploads, downloads, project operations, and other non-generation RPCs on existing transports.
- Add a narrowly scoped extension operation for the fixed Flow batchexecute endpoint. Do not permit arbitrary URLs or arbitrary RPC IDs.
- Use the extension client associated with the job browser profile and the existing profile-operation lock.
- If no suitable extension transport is attached, return a clear generation submission error rather than silently sending directly through Go HTTP.

## Implementation steps

1. Add an injectable response transport to the Go batchexecute POST path. Pass the built URL, form body, and required request headers; preserve existing retry logic and httpx.Response handling.
2. Add a typed CDP FlowRPC operation that sends request data to the extension and decodes status and response text.
3. Implement flow.rpc in the extension. Validate RPC allowlist, fixed Flow origin/path, method, and body shape; execute fetch in the Flow page MAIN world with browser credentials; return status and response text.
4. Advertise flow.rpc in ping and update the Flow bridge documentation.
5. Wire Go generation clients to the account-matched CDP client and WithProfileOperation, retaining direct HTTP for non-generation calls.
6. Review changed call sites, retries, ambiguous submission behavior, profile locking, and extension validation. Do not run tests; the user will test manually.

## Acceptance criteria

- Go still owns the generation envelope, payload, and response parsing.
- The browser sends the fixed Flow batchexecute POST with the attached profile credentials.
- Image and current video generation RPC variants use the browser path when extension is attached.
- Non-generation RPC traffic remains on its existing transport.
- flow.rpc cannot target arbitrary destinations.
- Browser transport failures are surfaced as ambiguous submission errors when Flow may have received the request.
- No automated tests are run; provide manual test points to the user.

