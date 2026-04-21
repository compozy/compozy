import "@testing-library/jest-dom/vitest";

import { afterEach } from "vitest";

afterEach(() => {
  if (typeof document !== "undefined") {
    document.cookie.split(";").forEach(entry => {
      const name = entry.split("=")[0]?.trim();
      if (name) {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
      }
    });
  }
  if (typeof window !== "undefined") {
    try {
      window.sessionStorage.clear();
      window.localStorage.clear();
    } catch {
      // ignore — jsdom may disable storage in some environments
    }
  }
});
