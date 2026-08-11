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

  const stages = Object.freeze({
    download: Object.freeze({ label: "Downloading runtime" }),
    verify: Object.freeze({ label: "Verifying runtime" }),
    install: Object.freeze({ label: "Installing runtime" }),
    start: Object.freeze({ label: "Starting runtime" }),
    ready: Object.freeze({ label: "Runtime ready" }),
  });

  const title = document.getElementById("boot-title");
  const detail = document.getElementById("boot-detail");
  const progress = document.getElementById("boot-progress");
  const progressLabel = document.getElementById("boot-progress-label");
  const progressAttempt = document.getElementById("boot-progress-attempt");
  const progressBar = document.getElementById("boot-progress-bar");
  const action = document.getElementById("boot-action");
  const controls = document.getElementById("boot-controls");
  const updateButton = document.getElementById("boot-update");
  const retryButton = document.getElementById("boot-retry");
  const diagnoseButton = document.getElementById("boot-diagnose");
  const result = document.getElementById("boot-result");
  const technical = document.getElementById("boot-technical");
  const diagnosticControls = document.getElementById("boot-diagnostic-controls");
  const copyButton = document.getElementById("boot-copy");
  const exportButton = document.getElementById("boot-export");
  const errorCodeRow = document.getElementById("boot-error-code-row");
  const errorCode = document.getElementById("boot-error-code");
  const currentErrorRow = document.getElementById("boot-current-error-row");
  const currentError = document.getElementById("boot-current-error");
  const previousCrashRow = document.getElementById("boot-previous-crash-row");
  const previousCrash = document.getElementById("boot-previous-crash");
  const phaseRow = document.getElementById("boot-phase-row");
  const phase = document.getElementById("boot-phase");
  const bootIDRow = document.getElementById("boot-boot-id-row");
  const bootID = document.getElementById("boot-boot-id");
  const appVersionRow = document.getElementById("boot-app-version-row");
  const appVersion = document.getElementById("boot-app-version");
  const runtimeVersionRow = document.getElementById("boot-runtime-version-row");
  const runtimeVersion = document.getElementById("boot-runtime-version");
  const runtimeOwnerRow = document.getElementById("boot-runtime-owner-row");
  const runtimeOwner = document.getElementById("boot-runtime-owner");
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

  const validPercentage = value =>
    Number.isInteger(value) && value >= 0 && value <= 100 ? value : null;

  const validAttempt = value =>
    Number.isInteger(value) && value >= 1 && value <= 3 ? value : null;

  const setResult = (message, level = "status") => {
    const text = safeText(message);
    const urgent = level === "error";
    result.textContent = text;
    result.hidden = text.length === 0;
    result.dataset.level = text.length === 0 ? "" : level;
    result.setAttribute("aria-live", urgent ? "assertive" : "polite");
    result.setAttribute("role", urgent ? "alert" : "status");
  };

  const setDiagnosticRow = (row, value, output) => {
    const text = safeText(value);
    output.textContent = text;
    row.hidden = text.length === 0;
    return text;
  };

  const clearProgress = () => {
    progress.hidden = true;
    progress.removeAttribute("aria-busy");
    progress.removeAttribute("data-stage");
    progressLabel.textContent = "";
    progressAttempt.textContent = "";
    progressAttempt.hidden = true;
    progressBar.hidden = true;
    progressBar.removeAttribute("aria-busy");
    progressBar.removeAttribute("aria-labelledby");
    progressBar.removeAttribute("aria-valuetext");
    progressBar.removeAttribute("value");
  };

  const renderProgress = input => {
    const stage = Object.hasOwn(stages, input.stage) ? input.stage : "";
    const attempt = validAttempt(input.attempt);
    if (stage.length === 0 && attempt === null) {
      clearProgress();
      return;
    }

    const label = stage.length > 0 ? stages[stage].label : stages.start.label;
    const percentage = stage === "download" ? validPercentage(input.pct) : null;
    const labelText = percentage === null ? label : `${label}: ${percentage}%`;
    const complete = stage === "ready";

    progress.hidden = false;
    progress.dataset.stage = stage;
    progress.setAttribute("aria-busy", complete ? "false" : "true");
    progressLabel.textContent = labelText;
    progressAttempt.textContent = attempt === null ? "" : `Attempt ${attempt}`;
    progressAttempt.hidden = attempt === null;
    progressBar.hidden = complete;
    progressBar.setAttribute("aria-labelledby", "boot-progress-label");
    progressBar.setAttribute("aria-valuetext", labelText);
    progressBar.setAttribute("aria-busy", complete ? "false" : "true");
    if (percentage === null) {
      progressBar.removeAttribute("value");
      return;
    }
    progressBar.value = percentage;
  };

  const describeRuntimeOwner = value => {
    if (value === true) return "Managed by CompozyOS";
    if (value === false) return "External";
    return "";
  };

  const describePreviousCrash = value => {
    if (value === null || typeof value !== "object") return "";
    const details = [
      safeText(value.boot_id),
      safeText(value.boot_phase),
      safeText(value.started_at),
    ].filter(part => part.length > 0);
    return details.join("; ");
  };

  const reportFrom = response => {
    if (response === null || typeof response !== "object" || response.schema_version !== 1) {
      throw new Error("Diagnostics could not be loaded.");
    }
    return response;
  };

  const clearDiagnostics = () => {
    technical.hidden = true;
    technical.open = false;
    diagnosticControls.hidden = true;
    currentErrorRow.dataset.error = "false";
    for (const [row, output] of [
      [errorCodeRow, errorCode],
      [currentErrorRow, currentError],
      [previousCrashRow, previousCrash],
      [phaseRow, phase],
      [bootIDRow, bootID],
      [appVersionRow, appVersion],
      [runtimeVersionRow, runtimeVersion],
      [runtimeOwnerRow, runtimeOwner],
    ]) {
      row.hidden = true;
      output.textContent = "";
    }
    configureButton(copyButton, { label: "Copy diagnostics", primary: false, visible: false });
    configureButton(exportButton, { label: "Export diagnostics", primary: false, visible: false });
  };

  const showDiagnostics = response => {
    const report = reportFrom(response);
    const error = setDiagnosticRow(
      currentErrorRow,
      report.current_error?.safe_message,
      currentError
    );
    currentErrorRow.dataset.error = error.length > 0 ? "true" : "false";
    const retainedErrorCode = errorCodeRow.hidden ? "" : errorCode.textContent;
    setDiagnosticRow(errorCodeRow, report.current_error?.code || retainedErrorCode, errorCode);
    setDiagnosticRow(previousCrashRow, describePreviousCrash(report.previous_crash), previousCrash);
    setDiagnosticRow(phaseRow, report.boot_phase, phase);
    setDiagnosticRow(bootIDRow, report.boot_id, bootID);
    setDiagnosticRow(appVersionRow, report.app_version, appVersion);
    setDiagnosticRow(runtimeVersionRow, report.runtime_version, runtimeVersion);
    setDiagnosticRow(runtimeOwnerRow, describeRuntimeOwner(report.runtime_owned), runtimeOwner);
    technical.hidden = false;
    diagnosticControls.hidden = false;
    configureButton(copyButton, { label: "Copy diagnostics", primary: false, visible: true });
    configureButton(exportButton, { label: "Export diagnostics", primary: false, visible: true });
  };

  const showPayloadError = error => {
    const code = setDiagnosticRow(errorCodeRow, error?.code, errorCode);
    if (code.length > 0) technical.hidden = false;
  };

  const configureButton = (button, options) => {
    nextButtonGeneration(button);
    button.hidden = !options.visible;
    button.disabled = false;
    button.removeAttribute("aria-busy");
    button.removeAttribute("aria-label");
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

  const runControl = async (button, method, params, pendingLabel, onSuccess) => {
    const generation = nextButtonGeneration(button);
    const label = button.dataset.label;
    button.style.inlineSize = `${button.getBoundingClientRect().width}px`;
    button.disabled = true;
    button.setAttribute("aria-busy", "true");
    button.setAttribute("aria-label", `${label}: ${pendingLabel}`);
    button.textContent = pendingLabel;
    setResult("");
    try {
      const response = await invoke(method, params);
      onSuccess(response);
    } catch (error) {
      setResult(controlError(error), "error");
    } finally {
      if (buttonGenerations.get(button) === generation) {
        button.disabled = false;
        button.removeAttribute("aria-busy");
        button.removeAttribute("aria-label");
        button.textContent = label;
        button.style.removeProperty("inline-size");
      }
    }
  };

  updateButton.addEventListener("click", () => {
    const target = updateTarget(updateButton.dataset.target);
    if (target) {
      void runControl(updateButton, "update.apply", { target }, `Updating ${target}…`, () => {
        setResult("Update started.", "success");
      });
    }
  });

  retryButton.addEventListener("click", () => {
    void runControl(retryButton, "retry", {}, "Retrying…", response => {
      if (response?.accepted !== true) throw new Error("The retry could not be started.");
      setResult("Retry started.", "success");
    });
  });

  diagnoseButton.addEventListener("click", () => {
    void runControl(diagnoseButton, "diagnose", {}, "Loading…", response => {
      showDiagnostics(response);
      setResult("Technical details loaded.", "success");
    });
  });

  copyButton.addEventListener("click", () => {
    void runControl(copyButton, "copy_diagnostics", {}, "Copying…", response => {
      if (response?.copied !== true) throw new Error("Diagnostics could not be copied.");
      setResult("Diagnostics copied.", "success");
    });
  });

  exportButton.addEventListener("click", () => {
    void runControl(
      exportButton,
      "export_diagnostics",
      { consent: true },
      "Exporting…",
      response => {
        if (safeText(response?.bundle_path).length === 0 || !Number.isFinite(response?.bytes)) {
          throw new Error("Diagnostics could not be exported.");
        }
        setResult("Diagnostics export created.", "success");
      }
    );
  });

  const render = payload => {
    const input = payload !== null && typeof payload === "object" ? payload : {};
    const state = Object.hasOwn(states, input.state) ? input.state : "error";
    const target = updateTarget(input.target);
    const showUpdate = target.length > 0 && state !== "updating";
    const showRetry = input.retry === true;
    const showDiagnose = input.diagnose === true;
    const primaryAction = showUpdate || showRetry;
    const detailText =
      safeText(input.message) || (state === "error" ? safeText(input.error?.safe_message) : "");
    const actionText = primaryAction ? "" : safeText(input.action);

    document.documentElement.dataset.state = state;
    title.textContent = states[state].title;
    title.setAttribute("aria-live", state === "error" ? "assertive" : "polite");
    detail.textContent = detailText;
    detail.hidden = detailText.length === 0;
    detail.setAttribute("aria-live", state === "error" ? "assertive" : "polite");
    detail.setAttribute("role", state === "error" ? "alert" : "status");
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
      label: "Load diagnostics",
      primary: false,
      visible: showDiagnose,
    });
    controls.hidden = !showUpdate && !showRetry && !showDiagnose;
    setResult("");
    clearDiagnostics();
    showPayloadError(input.error);
    renderProgress(input);
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
