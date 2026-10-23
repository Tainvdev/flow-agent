// chrome.scripting serializes this function, so it cannot refer to module scope.
export async function prepareRpc(engineBody, label = 'image', requestID = '') {
  const requestedRpc = label === 'image' ? 'ogiZ0b' : null;
  const log = (step, details = {}) => console.info(`[Flow Bridge ${label} RPC]`, step, details);
  requestID = String(requestID || '');
  if (!requestID) throw new Error('A unique Flow RPC request ID is required');
  const states = window.__flowBridgeRpcStates || (window.__flowBridgeRpcStates = Object.create(null));
  if (states[requestID]) throw new Error('This Flow RPC request ID is already present in this tab');

  const engineForm = new URLSearchParams(engineBody);
  const engineEnvelope = JSON.parse(engineForm.get('f.req') || '');
  const engineCall = engineEnvelope?.[0]?.[0];
  const rpcID = String(engineCall?.[0] || '');
  if (!Array.isArray(engineCall) || (requestedRpc && rpcID !== requestedRpc) ||
      (label === 'video' && !['YhhmEf', 'nprQif', 'eb1hJf', 'MZZa6b', 'jIps6'].includes(rpcID))) {
    throw new Error(`Engine body is not an allowlisted ${label} envelope`);
  }
  // Video CAPTCHA is shared by the video RPC family. Use the known browser
  // request as the token source, while keeping the RPC/body prepared by Go as
  // the request that is ultimately submitted.
  const captureRpcID = label === 'video' ? 'YhhmEf' : rpcID;
  const engineArgument = typeof engineCall[1] === 'string'
    ? JSON.parse(engineCall[1]) : engineCall[1];
  const captchaSlots = (argument) => {
    if (rpcID === 'ogiZ0b') {
      const slots = [argument?.[3]?.[10]];
      for (const request of argument?.[1] || []) slots.push(request?.[7]?.[10]);
      return slots;
    }
    return [argument?.[1]?.[10]];
  };
  const engineCaptchaSlots = captchaSlots(engineArgument);
  if (engineCaptchaSlots.some((slot) => !Array.isArray(slot) || !slot.length) ||
      (label === 'image' && (!Array.isArray(engineArgument?.[1]) || !engineArgument[1].length))) {
    throw new Error(`Engine body has no ${label} CAPTCHA slots`);
  }

  const desiredMode = label === 'image' ? 'Hình ảnh' : 'Video';
  const normalizedText = (value) => String(value || '')
    .replace(/\s+/g, ' ')
    .trim()
    .toLocaleLowerCase();
  const modeText = normalizedText(desiredMode);
  const findModeItem = () => {
    for (const container of document.querySelectorAll('.settings-content')) {
      const candidates = [container, ...container.querySelectorAll('*')];
      const exact = candidates.find((element) => normalizedText(element.textContent) === modeText);
      if (exact) return exact;
    }
    return null;
  };
  const summary = document.querySelector('.settings-summary');
  if (!summary) throw new Error('Flow settings summary was not found');
  summary.click();
  const modeDeadline = Date.now() + 2000;
  let modeItem = findModeItem();
  while (!modeItem && Date.now() < modeDeadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
    modeItem = findModeItem();
  }
  if (!modeItem) throw new Error(`Flow ${desiredMode} mode item was not found`);
  modeItem.click();
  log('generation mode selected', { mode: desiredMode });

  log('engine payload parsed', {
    rpc: rpcID,
    requests: Array.isArray(engineArgument?.[0]) ? engineArgument[0].length :
      (Array.isArray(engineArgument?.[1]) ? engineArgument[1].length : 1),
    bodyCharacters: engineBody.length,
    hasAtField: engineForm.has('at'),
    body: engineBody,
  });

  const originalFetch = window.fetch;
  const state = { id: requestID, label, sent: false, done: false, originalFetch, result: null };
  states[requestID] = state;

  const isTargetRequest = (url, method) => {
    try {
      const target = new URL(url, location.href);
      return method.toUpperCase() === 'POST' &&
        target.origin === 'https://flow.google.com' &&
        target.pathname === '/_/AiSandboxAngularFrontend/data/batchexecute' &&
        target.searchParams.get('rpcids') === captureRpcID;
    } catch {
      return false;
    }
  };

  const toFormBody = (body) => {
    if (typeof body === 'string') return body;
    if (body instanceof URLSearchParams) return body.toString();
    if (typeof FormData !== 'undefined' && body instanceof FormData) {
      if ([...body.values()].some((value) => typeof value !== 'string')) {
        throw new Error(`${label} request FormData contains a file`);
      }
      return new URLSearchParams(body).toString();
    }
    throw new Error(`Unsupported ${label} request body type`);
  };

  const wrappedFetch = async function (input, init) {
    const url = input instanceof Request ? input.url : String(input);
    const method = init?.method || (input instanceof Request ? input.method : 'GET');
    if (!isTargetRequest(url, method)) return originalFetch.call(this, input, init);

    // Only one user-generated media request belongs to this flow.rpc call.
    window.fetch = originalFetch;
    let receivedResponse = null;
    try {
      const sourceBody = toFormBody(init?.body === undefined && input instanceof Request
        ? await input.clone().text()
        : init?.body);
      log('Flow source request captured', {
        rpc: captureRpcID,
        engineRpc: rpcID,
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
      if (!Array.isArray(rpcCall) || rpcCall[0] !== captureRpcID) {
        throw new Error(`Observed request is not a ${captureRpcID} envelope`);
      }
      const argument = typeof rpcCall[1] === 'string' ? JSON.parse(rpcCall[1]) : rpcCall[1];
      const observedCaptchaSlots = captchaSlots(argument);
      const captcha = observedCaptchaSlots.map((slot) => slot?.[0])
        .find((value) => typeof value === 'string' && value);
      const at = sourceForm.get('at');
      if (typeof captcha !== 'string' || !captcha || !at) {
        throw new Error(`Observed ${label} request has no CAPTCHA or at token`);
      }
      log('request tokens extracted', { captcha, at, captchaLength: captcha.length, atLength: at.length });

      for (const slot of engineCaptchaSlots) slot[0] = captcha;
      engineCall[1] = JSON.stringify(engineArgument);
      engineForm.set('f.req', JSON.stringify(engineEnvelope));
      engineForm.set('at', at);
      const replacement = engineForm.toString();
      const targetURL = new URL(url, location.href);
      targetURL.searchParams.set('rpcids', rpcID);
      log('engine body populated with captured tokens', {
        rpc: rpcID,
        capturedFromRpc: captureRpcID,
        captchaSlotsReplaced: engineCaptchaSlots.length,
        atReplaced: true,
        bodyCharacters: replacement.length,
        body: replacement,
      });
      const headers = new Headers(input instanceof Request ? input.headers : undefined);
      if (init?.headers) new Headers(init.headers).forEach((value, name) => headers.set(name, value));
      headers.set('Content-Type', 'application/x-www-form-urlencoded;charset=UTF-8');
      if (input instanceof Request) {
        input = new Request(targetURL, {
          method: input.method,
          headers,
          body: replacement,
          credentials: input.credentials,
          mode: input.mode,
          cache: input.cache,
          redirect: input.redirect,
          referrer: input.referrer,
          referrerPolicy: input.referrerPolicy,
          integrity: input.integrity,
          keepalive: input.keepalive,
        });
        init = undefined;
      } else {
        init = { ...init, body: replacement, headers };
      }
      log('request body replaced', {
        rpc: rpcID,
        method: method.toUpperCase(),
        url: targetURL.toString(),
        capturedFromUrl: url,
        bodyCharacters: replacement.length,
        body: replacement,
        contentType: headers.get('Content-Type'),
        headers: Object.fromEntries(headers.entries()),
        credentials: input instanceof Request ? input.credentials : undefined,
        preservesOriginalRequest: input instanceof Request,
      });
      log('sending modified request to Flow', {
        rpc: rpcID,
        method: method.toUpperCase(),
        url: targetURL.toString(),
        body: replacement,
      });
      const responsePromise = originalFetch.call(this, input instanceof Request ? input : targetURL.toString(), init);
      state.sent = true;
      log('modified request dispatched', { requestId: requestID, rpc: rpcID });
      const response = await responsePromise;
      receivedResponse = response;
      const responseText = await response.clone().text();
      state.result = {
        status: response.status,
        text: responseText,
        reqid: Number(targetURL.searchParams.get('_reqid')) || null,
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
      console.error(`[Flow Bridge ${label} RPC] interception failed`, error);
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
