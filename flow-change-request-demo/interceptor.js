(() => {
  "use strict";

  const allowedHosts = new Set(["flow.google.com"]);
  const targetRpc = "ogiZ0b";

  // Captured payload template; supply fresh CAPTCHA and at tokens before use.
  const hardcodedPayloadTemplate = "f.req=%5B%5B%5B%22ogiZ0b%22%2C%22%5Bnull%2C%5B%5Bnull%2Cnull%2Cnull%2C1021862534%2C1%2C%5C%22BELUGA%5C%22%2Cnull%2C%5Bnull%2C22%2Cnull%2Cnull%2Cnull%2C%5C%2278e84224-51c6-44e6-ab3c-1033351153bf%5C%22%2Cnull%2Cnull%2Cnull%2Cnull%2C%5B%5C%22Captcha_thay_the%5C%22%2C1%5D%5D%2C%5B%5B%5B%5C%22c%E1%BA%A7u%20v%E1%BB%93ng%5C%22%5D%5D%5D%2Cnull%2Cnull%2Cnull%2C%5C%2216C59E6D-F85C-499E-A3B4-6174447B1E84%5C%22%2C%5C%2232035146-BB11-4E6F-9FBF-F7099814DAA5%5C%22%5D%5D%2C1%2C%5Bnull%2C22%2Cnull%2Cnull%2Cnull%2C%5C%2278e84224-51c6-44e6-ab3c-1033351153bf%5C%22%2Cnull%2Cnull%2Cnull%2Cnull%2C%5B%5C%22Captcha_thay_the%5C%22%2C1%5D%5D%2C%5B%5C%2248582E3D-7299-43B6-A439-B7578DA665BC%5C%22%5D%5D%22%2Cnull%2C%22generic%22%5D%5D%5D\u0026at=__AT_TOKEN__\u0026";

  if (!allowedHosts.has(location.hostname)) return;

  function isTargetRequest(url, method) {
    try {
      const parsedUrl = new URL(url, location.href);
      return (
        parsedUrl.origin === location.origin &&
        parsedUrl.searchParams.get("rpcids") === targetRpc &&
        method.toUpperCase() === "POST"
      );
    } catch {
      return false;
    }
  }

  // Extract the single-use CAPTCHA token from a batchexecute form body.
  // The outer f.req value is an RPC envelope; its argument is JSON encoded
  // again as a string. In the image RPC, the token is [3][10][0], with a
  // request-local copy at [1][0][7][10][0].
  function formBodyToString(body) {
    if (typeof body === "string") return body;
    if (body instanceof URLSearchParams) return body.toString();
    if (typeof FormData !== "undefined" && body instanceof FormData) {
      for (const [, value] of body.entries()) {
        if (typeof value !== "string") {
          console.warn("[Request Lab] FormData contains a file; cannot convert safely");
          return null;
        }
      }
      return new URLSearchParams(body).toString();
    }
    console.warn("[Request Lab] Unsupported body representation", {
      type: body === null ? "null" : typeof body,
      constructor: body?.constructor?.name ?? null
    });
    return null;
  }

  function extractCaptchaToken(body) {
    const formBody = formBodyToString(body);
    if (formBody === null || !formBody) {
      console.warn("[Request Lab] Cannot inspect body", {
        bodyType: body === null ? "null" : typeof body
      });
      return null;
    }

    try {
      let envelope;
      if (formBody.includes("f.req=")) {
        const encoded = new URLSearchParams(formBody).get("f.req");
        if (!encoded) {
          console.warn("[Request Lab] Form body has no f.req field");
          return null;
        }
        envelope = JSON.parse(encoded);
      } else {
        envelope = JSON.parse(formBody);
      }

      const rpcCall = envelope?.[0]?.[0];
      if (!Array.isArray(rpcCall) || rpcCall[0] !== targetRpc) {
        console.warn("[Request Lab] f.req RPC did not match", {
          expected: targetRpc,
          actual: Array.isArray(rpcCall) ? rpcCall[0] : null
        });
        return null;
      }

      const argument = typeof rpcCall[1] === "string"
        ? JSON.parse(rpcCall[1])
        : rpcCall[1];
      const token =
        argument?.[3]?.[10]?.[0] ??
        argument?.[1]?.[0]?.[7]?.[10]?.[0];
      if (typeof token !== "string" || token.length === 0) {
        console.warn("[Request Lab] RPC matched, but CAPTCHA token slot was empty");
        return null;
      }
      console.info("[Request Lab] CAPTCHA token extracted", {
        rpc: targetRpc,
        length: token.length
      });
      return token;
    } catch (error) {
      console.warn("[Request Lab] Could not parse request payload", error);
      return null;
    }
  }

  // Build a fresh copy of the captured form payload using the CAPTCHA token
  // extracted from the current request body. The current anti-CSRF `at` value
  // is copied too, since the captured one is session-specific.
  function buildPayloadWithExtractedCaptcha(sourceBody) {
    const captchaToken = extractCaptchaToken(sourceBody);
    const formBody = formBodyToString(sourceBody);
    if (!captchaToken || formBody === null) {
      console.warn("[Request Lab] Payload build stopped: CAPTCHA token unavailable");
      return null;
    }

    const sourceParams = new URLSearchParams(formBody);
    const atToken = sourceParams.get("at");
    if (!atToken) {
      console.warn("[Request Lab] Payload build stopped: form field at is missing");
      return null;
    }
    console.info("[Request Lab] at token found", { length: atToken.length });

    const encodedCaptcha = new URLSearchParams({ token: captchaToken })
      .toString()
      .slice("token=".length);
    const encodedAt = new URLSearchParams({ at: atToken })
      .toString()
      .slice("at=".length);

    const payload = hardcodedPayloadTemplate
      .split("Captcha_thay_the").join(encodedCaptcha)
      .split("__AT_TOKEN__").join(encodedAt);
    if (payload.includes("Captcha_thay_the") || payload.includes("__AT_TOKEN__")) {
      console.error("[Request Lab] Payload template still contains a placeholder");
      return null;
    }
    console.info("[Request Lab] Payload template populated", {
      characters: payload.length
    });
    return payload;
  }

  function transformBody(url, method, body, transport) {
    if (!isTargetRequest(url, method)) return null;
    console.info("[Request Lab] Target request matched", {
      transport,
      method,
      rpc: targetRpc
    });

    const payload = buildPayloadWithExtractedCaptcha(body);
    if (!payload) {
      console.warn("[Request Lab] Keeping original request body", {
        transport,
        rpc: targetRpc
      });
      return null;
    }

    console.info("[Request Lab] Replacing request body with captured payload", {
      transport,
      rpc: targetRpc
    });
    return payload;
  }

  // Intercept fetch requests whose body is supplied as a string in init.
  const originalFetch = window.fetch;
  window.fetch = async function (input, init) {
    try {
      const requestUrl = input instanceof Request ? input.url : String(input);
      const requestMethod = (
        init?.method || (input instanceof Request ? input.method : "GET")
      ).toUpperCase();
      let requestBody = init?.body;
      if (isTargetRequest(requestUrl, requestMethod)) {
        if (requestBody === undefined && input instanceof Request) {
          // Flow passes the body inside Request rather than init.body. Read a
          // clone so the original request remains available to fetch().
          requestBody = await input.clone().text();
          console.info("[Request Lab] Read body from cloned Request", {
            contentType: input.headers.get("Content-Type"),
            characters: requestBody.length
          });
        }

        console.info("[Request Lab] fetch body source", {
          inputType: input instanceof Request ? "Request" : typeof input,
          initBodyProvided: init != null && Object.prototype.hasOwnProperty.call(init, "body"),
          bodyType: requestBody?.constructor?.name ?? (requestBody === null ? "null" : typeof requestBody),
          contentType: input instanceof Request
            ? input.headers.get("Content-Type")
            : new Headers(init?.headers).get("Content-Type")
        });
      }
      const transformed = transformBody(requestUrl, requestMethod, requestBody, "fetch");

      if (transformed !== null) {
        const headers = new Headers(input instanceof Request ? input.headers : undefined);
        if (init?.headers) {
          new Headers(init.headers).forEach((value, name) => headers.set(name, value));
        }
        headers.set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8");

        if (input instanceof Request) {
          // Request bodies are immutable; build a replacement Request.
          input = new Request(input, { ...init, body: transformed, headers });
          init = undefined;
          console.info("[Request Lab] fetch Request object rebuilt with new body");
        } else {
          init = { ...init, body: transformed, headers };
          console.info("[Request Lab] fetch init body replaced");
        }
      }
    } catch (error) {
      console.warn("[Request Lab] fetch inspection failed:", error);
    }

    return originalFetch.call(this, input, init);
  };

  // Intercept XMLHttpRequest requests.
  const xhrMeta = new WeakMap();
  const originalOpen = XMLHttpRequest.prototype.open;
  const originalSend = XMLHttpRequest.prototype.send;
  const originalSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;

  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    xhrMeta.set(this, {
      method: String(method).toUpperCase(),
      url: new URL(String(url), location.href).href
    });
    return originalOpen.call(this, method, url, ...rest);
  };

  XMLHttpRequest.prototype.setRequestHeader = function (name, value) {
    const meta = xhrMeta.get(this);
    if (
      meta &&
      String(name).toLowerCase() === "content-type" &&
      isTargetRequest(meta.url, meta.method)
    ) {
      // Defer this header until send(), when we know whether the body changes.
      meta.contentTypes ??= [];
      meta.contentTypes.push(String(value));
      return;
    }
    return originalSetRequestHeader.call(this, name, value);
  };

  XMLHttpRequest.prototype.send = function (body) {
    const meta = xhrMeta.get(this);
    if (meta) {
      const transformed = transformBody(meta.url, meta.method, body, "xhr");
      if (transformed !== null) {
        body = transformed;
        originalSetRequestHeader.call(
          this,
          "Content-Type",
          "application/x-www-form-urlencoded;charset=UTF-8"
        );
        console.info("[Request Lab] XHR send body replaced");
      } else if (meta.contentTypes) {
        for (const contentType of meta.contentTypes) {
          originalSetRequestHeader.call(this, "Content-Type", contentType);
        }
      }
    }
    return originalSend.call(this, body);
  };

  console.info("[Request Lab] Active on", location.origin);
})();
