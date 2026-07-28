import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { onboardingCompletedFixture, onboardingIncompleteFixture } from "./fixtures";

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/onboarding", () =>
    HttpResponse.json({ onboarding: onboardingCompletedFixture })
  ),
  compozyApiMock.post("/api/onboarding/complete", () =>
    HttpResponse.json({ onboarding: onboardingCompletedFixture })
  ),
  compozyApiMock.delete("/api/onboarding", () =>
    HttpResponse.json({ onboarding: onboardingIncompleteFixture })
  ),
];
