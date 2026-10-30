import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { TooltipProvider, UIProvider } from "@compozy/ui";

import { GalleryApp } from "#gallery/app";

import "./styles.css";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <UIProvider reducedMotion="user">
        <TooltipProvider>
          <GalleryApp />
        </TooltipProvider>
      </UIProvider>
    </StrictMode>
  );
}
