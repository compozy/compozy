import { readFile } from "node:fs/promises";

import { expect, type Page } from "@playwright/test";

export async function completeOnboarding(product: Page): Promise<void> {
  const response = await product.request.post(
    new URL("/api/onboarding/complete", product.url()).toString()
  );
  expect(response.ok()).toBe(true);
  await product.reload({ waitUntil: "domcontentloaded" });
  await expect(product.getByRole("alertdialog", { name: "Set up CompozyOS" })).toHaveCount(0);
  await expect(product.getByTestId("os-desktop")).toBeVisible();
}

export async function readDiagnosticFile(path: string): Promise<string> {
  try {
    return (await readFile(path, "utf8")).trim() || "(empty)";
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return "(missing)";
    throw error;
  }
}
