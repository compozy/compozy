import { StrictMode } from "react";
import ReactDOM from "react-dom/client";

import { UIProvider } from "@compozy/ui";

import "@compozy/ui/tokens.css";

import { App } from "./app";
import "./styles.css";

const rootElement = document.getElementById("app");

if (!rootElement) {
  throw new Error("web bootstrap requires an #app root element");
}

ReactDOM.createRoot(rootElement).render(
  <StrictMode>
    <UIProvider>
      <App />
    </UIProvider>
  </StrictMode>
);
