async function runFlowAction() {
  const log = (step, details = {}) => console.info("[Flow Request Lab action]", step, details);
  const fail = (step, message, details = {}) => {
    console.error("[Flow Request Lab action]", step, details);
    return { ok: false, step, message, details };
  };

  log("started", { url: location.href });
  const editorMatches = document.querySelectorAll(".prosemirror-editor");
  const editorRoot = editorMatches[0];
  log("editor lookup", { selector: ".prosemirror-editor", count: editorMatches.length });
  if (!editorRoot) return fail("editor-not-found", "Could not find .prosemirror-editor.");

  const editor = editorRoot.matches("[contenteditable='true']")
    ? editorRoot
    : editorRoot.querySelector("[contenteditable='true']") || editorRoot;

  editor.focus();
  log("editor focused", {
    tag: editor.tagName,
    className: String(editor.className || ""),
    contentEditable: editor.isContentEditable,
    activeElementMatches: document.activeElement === editor
  });

  let inserted = false;
  const beforeLength = editor.textContent?.length || 0;
  try {
    const selection = window.getSelection();
    const range = document.createRange();
    range.selectNodeContents(editor);
    range.collapse(false);
    selection.removeAllRanges();
    selection.addRange(range);
    inserted = document.execCommand("insertText", false, "a");
  } catch (error) {
    return fail("typing-threw", "Could not insert 'a' into the editor.", {
      error: String(error)
    });
  }

  const afterLength = editor.textContent?.length || 0;
  log("text insertion", { execCommandResult: inserted, beforeLength, afterLength });
  if (!inserted && afterLength <= beforeLength) {
    return fail("typing-failed", "Editor was found, but inserting 'a' did not change its text.");
  }

  const generateMatches = document.querySelectorAll("flow-generate-icon-button > button");
  const generateButton = generateMatches[0];
  log("generate button lookup", {
    selector: "flow-generate-icon-button > button",
    count: generateMatches.length
  });
  if (!generateButton) {
    return fail("generate-button-not-found", "Typed 'a', but could not find the button inside flow-generate-icon-button.", {
      hostMatches: document.querySelectorAll("flow-generate-icon-button").length
    });
  }

  const disabledNow = () =>
    generateButton.disabled || generateButton.getAttribute("aria-disabled") === "true";
  const enableDeadline = Date.now() + 1500;
  while (disabledNow() && Date.now() < enableDeadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
  }

  const disabled = disabledNow();
  log("generate button state", {
    disabled,
    disabledAttribute: generateButton.hasAttribute("disabled"),
    matchesDisabledPseudo: generateButton.matches(":disabled"),
    ariaDisabled: generateButton.getAttribute("aria-disabled"),
    className: String(generateButton.className || ""),
    disabledFieldset: !!generateButton.closest("fieldset:disabled"),
    formFound: !!(generateButton.form || generateButton.closest("form")),
    editorTextLength: editor.textContent?.length || 0
  });
  if (disabled) {
    return fail("generate-button-disabled", "Typed 'a', but the Generate button stayed disabled.", {
      disabledAttribute: generateButton.hasAttribute("disabled"),
      matchesDisabledPseudo: generateButton.matches(":disabled"),
      disabledFieldset: !!generateButton.closest("fieldset:disabled"),
      formFound: !!(generateButton.form || generateButton.closest("form")),
      editorTextLength: editor.textContent?.length || 0
    });
  }

  generateButton.scrollIntoView({ block: "center", inline: "center" });
  const rect = generateButton.getBoundingClientRect();
  const clickPoint = {
    x: rect.left + rect.width / 2,
    y: rect.top + rect.height / 2
  };
  window.__flowRequestLabClickStatus = { seen: false, trusted: null };
  generateButton.addEventListener("click", (event) => {
    window.__flowRequestLabClickStatus = {
      seen: true,
      trusted: event.isTrusted,
      target: event.target?.tagName || null
    };
    log("trusted click observed", window.__flowRequestLabClickStatus);
  }, { once: true, capture: true });
  log("dispatching debugger mouse input", clickPoint);
  return {
    ok: true,
    message: "Prompt prepared; sending mouse input to Generate.",
    clickPoint
  };
}

const runButton = document.getElementById("run");
const result = document.getElementById("result");

runButton.addEventListener("click", async () => {
  runButton.disabled = true;
  result.className = "";
  result.textContent = "Running on the active tab…";

  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    console.info("[Flow Request Lab popup] active tab", {
      id: tab?.id,
      url: tab?.url
    });
    if (!tab?.id || !tab.url?.startsWith("https://flow.google.com/")) {
      throw new Error("Open a Flow tab first.");
    }

    const [execution] = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      world: "MAIN",
      func: runFlowAction
    });

    const actionResult = execution?.result;
    console.info("[Flow Request Lab popup] page result", actionResult);
    if (!actionResult?.ok) throw new Error(actionResult?.message || "Action failed.");

    const debuggee = { tabId: tab.id };
    let attached = false;
    try {
      await chrome.debugger.attach(debuggee, "1.3");
      attached = true;
      await chrome.debugger.sendCommand(debuggee, "Input.dispatchMouseEvent", {
        type: "mouseMoved",
        x: actionResult.clickPoint.x,
        y: actionResult.clickPoint.y
      });
      await chrome.debugger.sendCommand(debuggee, "Input.dispatchMouseEvent", {
        type: "mousePressed",
        x: actionResult.clickPoint.x,
        y: actionResult.clickPoint.y,
        button: "left",
        clickCount: 1
      });
      await chrome.debugger.sendCommand(debuggee, "Input.dispatchMouseEvent", {
        type: "mouseReleased",
        x: actionResult.clickPoint.x,
        y: actionResult.clickPoint.y,
        button: "left",
        clickCount: 1
      });
    } finally {
      if (attached) await chrome.debugger.detach(debuggee).catch((error) => {
        console.warn("[Flow Request Lab popup] debugger detach failed", error);
      });
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
    const [statusExecution] = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      world: "MAIN",
      func: () => ({
        clickStatus: window.__flowRequestLabClickStatus || null,
        editorText: document.querySelector(".prosemirror-editor")?.innerText || "",
        buttonDisabled: document.querySelector("flow-generate-icon-button > button")?.disabled ?? null
      })
    });
    const clickStatus = statusExecution?.result;
    console.info("[Flow Request Lab popup] debugger activation result", clickStatus);
    if (!clickStatus?.clickStatus?.seen) {
      throw new Error("Mouse input was sent, but the Generate click handler did not run.");
    }
    result.textContent = `${actionResult.message} Click trusted: ${clickStatus.clickStatus.trusted}.`;
    result.className = "ok";
  } catch (error) {
    console.error("[Flow Request Lab popup] injection failed", error);
    result.textContent = error?.message || String(error);
    result.className = "error";
  } finally {
    runButton.disabled = false;
  }
});
