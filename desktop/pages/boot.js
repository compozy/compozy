(() => {
  "use strict";

  const states = Object.freeze({
    resolving: "Looking for CompozyOS",
    provisioning: "Preparing CompozyOS",
    starting: "Starting CompozyOS",
    attaching: "Connecting to CompozyOS",
    product: "CompozyOS is ready",
    updating: "Updating CompozyOS",
    disconnected: "CompozyOS is unavailable",
    skew: "CompozyOS needs an update",
    error: "CompozyOS could not start",
  });
  const stages = Object.freeze({
    download: "Downloading runtime",
    verify: "Verifying runtime",
    install: "Installing runtime",
    start: "Starting runtime",
    ready: "Runtime ready",
  });
  const element = id => {
    const value = document.getElementById(id);
    if (!value) throw new Error(`Missing boot element ${id}.`);
    return value;
  };
  const title = element("boot-title");
  const detail = element("boot-detail");
  const progress = element("boot-progress");
  const progressLabel = element("boot-progress-label");
  const progressAttempt = element("boot-progress-attempt");
  const progressBar = element("boot-progress-bar");
  const controls = element("boot-controls");
  const retry = element("boot-retry");
  const openLogs = element("boot-open-logs");
  const quit = element("boot-quit");
  const diagnose = element("boot-diagnose");
  const result = element("boot-result");
  const technical = element("boot-technical");
  const diagnostics = element("boot-diagnostics");
  const diagnosticControls = element("boot-diagnostic-controls");
  const copy = element("boot-copy");
  const exportButton = element("boot-export");

  const safeText = value => {
    if (typeof value !== "string") return "";
    return Array.from(value, character => {
      const codePoint = character.codePointAt(0) ?? 0;
      return codePoint <= 31 || codePoint === 127 ? " " : character;
    })
      .join("")
      .trim()
      .slice(0, 512);
  };

  const setResult = (message, level = "status") => {
    const text = safeText(message);
    result.textContent = text;
    result.hidden = text.length === 0;
    result.dataset.level = text ? level : "";
    result.setAttribute("role", level === "error" ? "alert" : "status");
    result.setAttribute("aria-live", level === "error" ? "assertive" : "polite");
  };

  const setButton = (button, visible) => {
    button.hidden = !visible;
    button.disabled = false;
    button.removeAttribute("aria-busy");
  };

  const renderProgress = input => {
    const stage =
      typeof input.stage === "string" && Object.hasOwn(stages, input.stage) ? input.stage : "";
    const attempt =
      Number.isInteger(input.attempt) && input.attempt >= 1 && input.attempt <= 3
        ? input.attempt
        : null;
    if (!stage && attempt === null) {
      progress.hidden = true;
      return;
    }
    const percentage =
      stage === "download" && Number.isInteger(input.pct) && input.pct >= 0 && input.pct <= 100
        ? input.pct
        : null;
    const label = stage ? stages[stage] : stages.start;
    const labelText = percentage === null ? label : `${label}: ${percentage}%`;
    progress.hidden = false;
    progress.dataset.stage = stage;
    progress.setAttribute("aria-busy", stage === "ready" ? "false" : "true");
    progressLabel.textContent = labelText;
    progressAttempt.textContent = attempt === null ? "" : `Attempt ${attempt}`;
    progressAttempt.hidden = attempt === null;
    progressBar.hidden = stage === "ready";
    progressBar.setAttribute("aria-valuetext", labelText);
    if (percentage === null) progressBar.removeAttribute("value");
    else progressBar.value = percentage;
  };

  const addDiagnostic = (label, value) => {
    const text = safeText(value);
    if (!text) return;
    const row = document.createElement("div");
    const term = document.createElement("dt");
    const description = document.createElement("dd");
    term.textContent = label;
    description.textContent = text;
    row.append(term, description);
    diagnostics.append(row);
  };

  const showDiagnostics = report => {
    if (!report || typeof report !== "object" || report.schema_version !== 1) {
      throw new Error("Diagnostics could not be loaded.");
    }
    diagnostics.replaceChildren();
    addDiagnostic("Error code", report.current_error?.code);
    addDiagnostic("Current error", report.current_error?.safe_message);
    addDiagnostic("Boot phase", report.boot_phase);
    addDiagnostic("Boot ID", report.boot_id);
    addDiagnostic("App version", report.app_version);
    addDiagnostic("Runtime version", report.runtime_version);
    addDiagnostic(
      "Runtime owner",
      report.runtime_owned === true
        ? "Managed by CompozyOS"
        : report.runtime_owned === false
          ? "External"
          : ""
    );
    technical.hidden = false;
    diagnosticControls.hidden = false;
  };

  const invoke = async (button, method, params, pendingLabel, onSuccess) => {
    const label = button.textContent;
    button.disabled = true;
    button.setAttribute("aria-busy", "true");
    button.textContent = pendingLabel;
    setResult("");
    try {
      const response = await window.compozyBoot.invoke(method, params);
      onSuccess(response);
    } catch (error) {
      setResult(
        error instanceof Error ? error.message : "The requested action could not be completed.",
        "error"
      );
    } finally {
      button.disabled = false;
      button.removeAttribute("aria-busy");
      button.textContent = label;
    }
  };

  retry.addEventListener(
    "click",
    () => void invoke(retry, "retry", {}, "Retrying…", () => setResult("Retry started.", "success"))
  );
  openLogs.addEventListener(
    "click",
    () =>
      void invoke(openLogs, "open_logs", {}, "Opening…", () => setResult("Logs opened.", "success"))
  );
  quit.addEventListener("click", () => void window.compozyBoot.invoke("quit", {}));
  diagnose.addEventListener(
    "click",
    () =>
      void invoke(diagnose, "diagnose", {}, "Loading…", response => {
        showDiagnostics(response);
        setResult("Technical details loaded.", "success");
      })
  );
  copy.addEventListener(
    "click",
    () =>
      void invoke(copy, "copy_diagnostics", {}, "Copying…", () =>
        setResult("Diagnostics copied.", "success")
      )
  );
  exportButton.addEventListener(
    "click",
    () =>
      void invoke(exportButton, "export_diagnostics", { consent: true }, "Exporting…", response => {
        if (!response || typeof response !== "object" || typeof response.bundle_path !== "string")
          throw new Error("Diagnostics could not be exported.");
        setResult("Diagnostics export created.", "success");
      })
  );

  const render = payload => {
    const input = payload && typeof payload === "object" ? payload : {};
    const state =
      typeof input.state === "string" && Object.hasOwn(states, input.state) ? input.state : "error";
    const isError = state === "error";
    document.documentElement.dataset.state = state;
    title.textContent = states[state];
    title.setAttribute("aria-live", isError ? "assertive" : "polite");
    const skewMessage =
      state !== "skew"
        ? ""
        : input.newer === true
          ? `Update CompozyOS to ${safeText(input.needed)} or newer before reconnecting to runtime ${safeText(input.runtime)}.`
          : `Runtime ${safeText(input.runtime)} does not satisfy ${safeText(input.needed)}. Repair or update the runtime before reconnecting.`;
    const message =
      safeText(input.message) || (isError ? safeText(input.error?.safe_message) : skewMessage);
    detail.textContent = message;
    detail.hidden = !message;
    detail.setAttribute("role", isError ? "alert" : "status");
    setButton(retry, isError);
    setButton(openLogs, isError);
    setButton(quit, isError);
    setButton(diagnose, isError);
    controls.hidden = !isError;
    technical.hidden = true;
    diagnosticControls.hidden = true;
    setResult("");
    renderProgress(input);
  };

  window.compozyBoot.onState(render);
  render({ state: "resolving" });
})();
