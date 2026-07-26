import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { onboardingCompletedFixture, onboardingIncompleteFixture } from "./fixtures";

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/onboarding", () =>
    HttpResponse.json({ onboarding: onboardingCompletedFixture })
  ),
  aghApiMock.post("/api/onboarding/complete", () =>
    HttpResponse.json({ onboarding: onboardingCompletedFixture })
  ),
  aghApiMock.delete("/api/onboarding", () =>
    HttpResponse.json({ onboarding: onboardingIncompleteFixture })
  ),
];
