// SPDX-License-Identifier: FSL-1.1-MIT

for (const form of document.querySelectorAll("form[data-download]")) {
  form.addEventListener("submit", async event => {
    event.preventDefault();
    const button = form.querySelector("button");
    const status = form.querySelector("[role=status]");
    button.disabled = true;
    status.textContent = "";
    try {
      const response = await fetch(form.action, { method: "POST" });
      if (!response.ok) throw new Error("Download failed");

      const data = URL.createObjectURL(await response.blob());
      const download = document.createElement("a");
      download.href = data;
      download.download = form.dataset.download;
      document.body.append(download);
      download.click();
      download.remove();
      setTimeout(() => URL.revokeObjectURL(data), 60_000);

      const id = response.headers.get("X-Compozy-Security-Event");
      if (id && /^\d+$/.test(id)) {
        const receipt = document.createElement("a");
        receipt.href = "/admin/events/" + id;
        receipt.textContent = "Security event " + id;
        status.append(receipt);
      }
    } catch {
      status.textContent = "Download failed. Try again.";
    } finally {
      button.disabled = false;
    }
  });
}
