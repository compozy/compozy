(() => {
  "use strict";

  const states = Object.freeze({
    resolving: Object.freeze({ title: "Looking for CompozyOS" }),
    provisioning: Object.freeze({ title: "Preparing CompozyOS" }),
    starting: Object.freeze({ title: "Starting CompozyOS" }),
    attaching: Object.freeze({ title: "Connecting to CompozyOS" }),
    product: Object.freeze({ title: "CompozyOS is ready" }),
    updating: Object.freeze({ title: "Updating CompozyOS" }),
    disconnected: Object.freeze({ title: "CompozyOS is unavailable" }),
    skew: Object.freeze({ title: "CompozyOS needs an update" }),
    error: Object.freeze({ title: "CompozyOS could not start" }),
  });

  const title = document.getElementById("boot-title");
  const detail = document.getElementById("boot-detail");
  const action = document.getElementById("boot-action");
  const controls = document.getElementById("boot-controls");
  const updateButton = document.getElementById("boot-update");
  const retryButton = document.getElementById("boot-retry");
  const diagnoseButton = document.getElementById("boot-diagnose");
  const result = document.getElementById("boot-result");
  const diagnostics = document.getElementById("boot-diagnostics");
  const appLogRow = document.getElementById("boot-app-log-row");
  const appLog = document.getElementById("boot-app-log");
  const runtimeLogRow = document.getElementById("boot-runtime-log-row");
  const runtimeLog = document.getElementById("boot-runtime-log");
  const lastErrorRow = document.getElementById("boot-last-error-row");
  const lastError = document.getElementById("boot-last-error");
  const buttonGenerations = new WeakMap();

  const nextButtonGeneration = button => {
    const generation = (buttonGenerations.get(button) ?? 0) + 1;
    buttonGenerations.set(button, generation);
    return generation;
  };

  const safeText = value => {
    if (typeof value !== "string") return "";
    return Array.from(value, character => {
      const codePoint = character.codePointAt(0);
      return codePoint <= 31 || codePoint === 127 ? " " : character;
    })
      .join("")
      .trim()
      .slice(0, 512);
  };

  const updateTarget = value => (value === "app" || value === "runtime" ? value : "");

  const setResult = (message, urgent = false) => {
    const text = safeText(message);
    result.textContent = text;
    result.hidden = text.length === 0;
    result.setAttribute("aria-live", urgent ? "assertive" : "polite");
  };

  const clearDiagnostics = () => {
    diagnostics.hidden = true;
    for (const row of [appLogRow, runtimeLogRow, lastErrorRow]) row.hidden = true;
    appLog.textContent = "";
    runtimeLog.textContent = "";
    lastError.textContent = "";
  };

  const showDiagnostics = payload => {
    const app = safeText(payload?.app_log);
    const runtime = safeText(payload?.runtime_log);
    const error = safeText(payload?.last_error?.safe_message);
    appLog.textContent = app;
    appLogRow.hidden = app.length === 0;
    runtimeLog.textContent = runtime;
    runtimeLogRow.hidden = runtime.length === 0;
    lastError.textContent = error;
    lastErrorRow.hidden = error.length === 0;
    diagnostics.hidden = app.length === 0 && runtime.length === 0 && error.length === 0;
  };

  const configureButton = (button, options) => {
    nextButtonGeneration(button);
    button.hidden = !options.visible;
    button.disabled = false;
    button.removeAttribute("aria-busy");
    button.style.removeProperty("inline-size");
    button.classList.toggle("boot__control--primary", options.primary);
    button.textContent = options.label;
    button.dataset.label = options.label;
  };

  const invoke = (method, params) => {
    if (typeof window.__TAURI__?.core?.invoke !== "function") {
      return Promise.reject(new Error("The app control is unavailable."));
    }
    return window.__TAURI__.core.invoke("shell_control", { method, params });
  };

  const controlError = error => {
    if (error !== null && typeof error === "object") {
      return safeText(error.message) || "The requested action could not be completed.";
    }
    return safeText(error) || "The requested action could not be completed.";
  };

  const runControl = async (button, method, params, pendingLabel) => {
    const generation = nextButtonGeneration(button);
    const label = button.dataset.label;
    button.style.inlineSize = `${button.getBoundingClientRect().width}px`;
    button.disabled = true;
    button.setAttribute("aria-busy", "true");
    button.textContent = pendingLabel;
    setResult("");
    try {
      const response = await invoke(method, params);
      if (method === "diagnose") {
        showDiagnostics(response);
        return;
      }
      setResult(method === "retry" ? "Retry accepted." : "Update started.");
    } catch (error) {
      setResult(controlError(error), true);
    } finally {
      if (buttonGenerations.get(button) === generation) {
        button.disabled = false;
        button.removeAttribute("aria-busy");
        button.textContent = label;
        button.style.removeProperty("inline-size");
      }
    }
  };

  updateButton.addEventListener("click", () => {
    const target = updateTarget(updateButton.dataset.target);
    if (target)
      void runControl(updateButton, "update.apply", { target }, `Updating ${target}\u2026`);
  });
  retryButton.addEventListener("click", () => {
    void runControl(retryButton, "retry", {}, "Retrying\u2026");
  });
  diagnoseButton.addEventListener("click", () => {
    void runControl(diagnoseButton, "diagnose", {}, "Loading\u2026");
  });

  const render = payload => {
    const input = payload !== null && typeof payload === "object" ? payload : {};
    const state = Object.hasOwn(states, input.state) ? input.state : "error";
    const target = updateTarget(input.target);
    const showUpdate = target.length > 0 && state !== "updating";
    const showRetry =
      state === "error" ||
      input.retry === true ||
      input.recovery === true ||
      (state === "provisioning" && input.failed === true);
    const showDiagnose = state === "error" || input.diagnose === true;
    const primaryAction = showUpdate || showRetry;
    const detailText = safeText(input.message);
    const actionText = primaryAction ? "" : safeText(input.action);

    document.documentElement.dataset.state = state;
    document.documentElement.dataset.primaryAction = primaryAction ? "true" : "false";
    title.textContent = states[state].title;
    detail.textContent = detailText;
    detail.hidden = detailText.length === 0;
    action.textContent = actionText;
    action.hidden = actionText.length === 0;
    configureButton(updateButton, {
      label: target === "app" ? "Update app" : "Update runtime",
      primary: showUpdate,
      visible: showUpdate,
    });
    updateButton.dataset.target = target;
    configureButton(retryButton, {
      label: "Retry operation",
      primary: showRetry && !showUpdate,
      visible: showRetry,
    });
    configureButton(diagnoseButton, {
      label: "Show diagnostics",
      primary: false,
      visible: showDiagnose,
    });
    controls.hidden = !showUpdate && !showRetry && !showDiagnose;
    setResult("");
    clearDiagnostics();
    title.setAttribute("aria-live", state === "error" ? "assertive" : "polite");
    return state;
  };

  Object.defineProperty(window, "__COMPOZY_BOOT__", {
    configurable: false,
    enumerable: false,
    value: Object.freeze({ render }),
    writable: false,
  });

  render({ state: "resolving" });
})();
