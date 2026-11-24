const checkout = document.querySelector("[data-billing-redirect]");
if (checkout instanceof HTMLAnchorElement) window.location.replace(checkout.href);
