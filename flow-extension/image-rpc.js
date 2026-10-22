// chrome.scripting serializes this function, so it cannot refer to module scope.
export async function prepareImageRpc(engineBody) {
  const log = (step, details = {}) => console.info('[Flow Bridge image RPC]', step, details);
  const previous = window.__flowBridgeImageRpc;
  if (previous && !previous.done) throw new Error('An image RPC is already pending in this tab');

  const engineForm = new URLSearchParams(engineBody);
  const engineEnvelope = JSON.parse(engineForm.get('f.req') || '');
  const engineCall = engineEnvelope?.[0]?.[0];
  if (!Array.isArray(engineCall) || engineCall[0] !== 'ogiZ0b') {
    throw new Error('Engine body is not an ogiZ0b envelope');
  }
  const engineArgument = typeof engineCall[1] === 'string'
    ? JSON.parse(engineCall[1]) : engineCall[1];
  const contextToken = engineArgument?.[3]?.[10];
  const requests = engineArgument?.[1];
  if (!Array.isArray(contextToken) || !Array.isArray(requests) || !requests.length ||
      requests.some((request) => !Array.isArray(request?.[7]?.[10]))) {
    throw new Error('Engine body has no image CAPTCHA slots');
  }
  log('engine payload parsed', {
    rpc: 'ogiZ0b',
    requests: requests.length,
    bodyCharacters: engineBody.length,
    hasAtField: engineForm.has('at'),
    body: engineBody,
  });

  const originalFetch = window.fetch;
  const state = { done: false, originalFetch, result: null };
  window.__flowBridgeImageRpc = state;

  const isImageRequest = (url, method) => {
    try {
      const target = new URL(url, location.href);
      return method.toUpperCase() === 'POST' &&
        target.origin === 'https://flow.google.com' &&
        target.pathname === '/_/AiSandboxAngularFrontend/data/batchexecute' &&
        target.searchParams.get('rpcids') === 'ogiZ0b';
    } catch {
      return false;
    }
  };

  const toFormBody = (body) => {
    if (typeof body === 'string') return body;
    if (body instanceof URLSearchParams) return body.toString();
    if (typeof FormData !== 'undefined' && body instanceof FormData) {
      if ([...body.values()].some((value) => typeof value !== 'string')) {
        throw new Error('Image request FormData contains a file');
      }
      return new URLSearchParams(body).toString();
    }
    throw new Error('Unsupported image request body type');
  };

  const wrappedFetch = async function (input, init) {
    const url = input instanceof Request ? input.url : String(input);
    const method = init?.method || (input instanceof Request ? input.method : 'GET');
    if (!isImageRequest(url, method)) return originalFetch.call(this, input, init);

    // Only one user-generated image request belongs to this flow.rpc call.
    window.fetch = originalFetch;
    let receivedResponse = null;
    try {
      const sourceBody = toFormBody(init?.body === undefined && input instanceof Request
        ? await input.clone().text()
        : init?.body);
      log('Flow source request captured', {
        rpc: 'ogiZ0b',
        method: method.toUpperCase(),
        url,
        bodyType: init?.body === undefined && input instanceof Request ? 'Request' : typeof init?.body,
        bodyCharacters: sourceBody.length,
        body: sourceBody,
        headers: Object.fromEntries(new Headers(input instanceof Request ? input.headers : init?.headers).entries()),
        credentials: input instanceof Request ? input.credentials : undefined,
      });
      const sourceForm = new URLSearchParams(sourceBody);
      const envelope = JSON.parse(sourceForm.get('f.req') || '');
      const rpcCall = envelope?.[0]?.[0];
      if (!Array.isArray(rpcCall) || rpcCall[0] !== 'ogiZ0b') {
        throw new Error('Observed request is not an ogiZ0b envelope');
      }
      const argument = typeof rpcCall[1] === 'string' ? JSON.parse(rpcCall[1]) : rpcCall[1];
      const captcha = argument?.[3]?.[10]?.[0] ?? argument?.[1]?.[0]?.[7]?.[10]?.[0];
      const at = sourceForm.get('at');
      if (typeof captcha !== 'string' || !captcha || !at) {
        throw new Error('Observed image request has no CAPTCHA or at token');
      }
      log('request tokens extracted', { captcha, at, captchaLength: captcha.length, atLength: at.length });

      contextToken[0] = captcha;
      for (const request of requests) {
        const requestToken = request?.[7]?.[10];
        requestToken[0] = captcha;
      }
      engineCall[1] = JSON.stringify(engineArgument);
      engineForm.set('f.req', JSON.stringify(engineEnvelope));
      engineForm.set('at', at);
      const replacement = engineForm.toString();
      log('engine body populated with captured tokens', {
        rpc: 'ogiZ0b',
        requests: requests.length,
        captchaSlotsReplaced: requests.length + 1,
        atReplaced: true,
        bodyCharacters: replacement.length,
        body: replacement,
      });
      const headers = new Headers(input instanceof Request ? input.headers : undefined);
      if (init?.headers) new Headers(init.headers).forEach((value, name) => headers.set(name, value));
      headers.set('Content-Type', 'application/x-www-form-urlencoded;charset=UTF-8');
      if (input instanceof Request) {
        input = new Request(input, { ...init, body: replacement, headers });
        init = undefined;
      } else {
        init = { ...init, body: replacement, headers };
      }
      log('request body replaced', {
        rpc: 'ogiZ0b',
        method: method.toUpperCase(),
        url,
        bodyCharacters: replacement.length,
        body: replacement,
        contentType: headers.get('Content-Type'),
        headers: Object.fromEntries(headers.entries()),
        credentials: input instanceof Request ? input.credentials : undefined,
        preservesOriginalRequest: input instanceof Request,
      });
      log('sending modified request to Flow', {
        rpc: 'ogiZ0b',
        method: method.toUpperCase(),
        url,
        body: replacement,
      });
      const response = await originalFetch.call(this, input, init);
      receivedResponse = response;
      const responseText = await response.clone().text();
      state.result = {
        status: response.status,
        text: responseText,
        captcha_minted: true,
        reqid: Number(new URL(url, location.href).searchParams.get('_reqid')) || null,
      };
      state.done = true;
      log('Flow response received', {
        status: response.status,
        ok: response.ok,
        contentType: response.headers.get('content-type'),
        responseCharacters: responseText.length,
        response: responseText,
      });
      return response;
    } catch (error) {
      state.result = { error: String(error) };
      state.done = true;
      console.error('[Flow Bridge image RPC] interception failed', error);
      if (receivedResponse) return receivedResponse;
      throw error;
    }
  };
  state.wrappedFetch = wrappedFetch;
  window.fetch = wrappedFetch;

  try {
    const editorRoot = document.querySelector('.prosemirror-editor');
    const editor = editorRoot?.matches('[contenteditable="true"]')
      ? editorRoot : editorRoot?.querySelector('[contenteditable="true"]');
    if (!editor) throw new Error('Flow prompt editor was not found');
    editor.focus();
    const selection = window.getSelection();
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    selection.removeAllRanges();
    selection.addRange(range);
    const before = editor.textContent?.length || 0;
    const inserted = document.execCommand('insertText', false, 'a');
    if (!inserted && (editor.textContent?.length || 0) <= before) {
      throw new Error('Could not type into the Flow prompt editor');
    }

    const button = document.querySelector('flow-generate-icon-button > button');
    if (!button) throw new Error('Flow Generate button was not found');
    const deadline = Date.now() + 1500;
    while (button.disabled && Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    if (button.disabled) throw new Error('Flow Generate button stayed disabled');
    button.scrollIntoView({ block: 'center', inline: 'center' });
    const rect = button.getBoundingClientRect();
    if (!rect.width || !rect.height) throw new Error('Flow Generate button is not visible');
    state.clickSeen = false;
    button.addEventListener('click', (event) => {
      state.clickSeen = true;
      state.clickTrusted = event.isTrusted;
      log('Generate click observed', { trusted: event.isTrusted });
    }, { once: true, capture: true });
    const point = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
    log('ready for browser click', point);
    return point;
  } catch (error) {
    window.fetch = originalFetch;
    state.done = true;
    state.result = { error: String(error) };
    throw error;
  }
}
