const form = document.querySelector("#provider-form");
const challenge = document.querySelector("#turnstile");
const token = form.elements.namedItem("cf-turnstile-response");
const status = document.querySelector("#turnstile-error");
let widget;
let submitter;

function failed() {
  token.value = "";
  submitter = undefined;
  status.hidden = false;
}

form.addEventListener("submit", event => {
  if (token.value) return;
  event.preventDefault();
  if (submitter) return;
  if (!window.turnstile) {
    failed();
    return;
  }
  status.hidden = true;
  submitter = event.submitter ?? form.querySelector("button[name=provider]");
  window.turnstile.ready(() => {
    if (widget === undefined) {
      widget = window.turnstile.render(challenge, {
        sitekey: challenge.dataset.siteKey,
        action: challenge.dataset.action,
        execution: "execute",
        appearance: "interaction-only",
        "response-field": false,
        callback(response) {
          token.value = response;
          form.requestSubmit(submitter);
        },
        "error-callback": failed,
        "expired-callback": failed,
        "timeout-callback": failed,
      });
    } else {
      window.turnstile.reset(widget);
    }
    window.turnstile.execute(widget);
  });
});
