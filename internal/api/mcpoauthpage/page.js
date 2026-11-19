const copy = document.querySelector("[data-copy]");
const diagnostic = document.querySelector("[data-diagnostic]");
if (copy && diagnostic) {
  copy.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(diagnostic.textContent || "");
      copy.textContent = "Copied";
    } catch {
      copy.textContent = "Couldn’t copy";
    }
  });
}
