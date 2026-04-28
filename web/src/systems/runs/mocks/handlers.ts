import { http, HttpResponse, type HttpHandler } from "msw";

import type { Run, RunSnapshot, RunTranscript } from "../types";
import {
  dispatchedRunFixture,
  runSnapshotFixture,
  runTranscriptFixture,
  runsFixture,
} from "./fixtures";

export interface RunHandlerOptions {
  runs?: Run[];
  snapshot?: RunSnapshot;
  transcript?: RunTranscript;
  startRun?: Run;
}

export function createRunHandlers(options: RunHandlerOptions = {}): HttpHandler[] {
  const runs = options.runs ?? runsFixture;
  const snapshot = options.snapshot ?? runSnapshotFixture;
  const transcript = options.transcript ?? runTranscriptFixture;
  const startRun = options.startRun ?? dispatchedRunFixture;

  return [
    http.get("/api/runs", () => HttpResponse.json({ runs })),
    http.get("/api/runs/:run_id", () => HttpResponse.json({ run: runs[0] ?? startRun })),
    http.get("/api/runs/:run_id/snapshot", () => HttpResponse.json(snapshot)),
    http.get("/api/runs/:run_id/transcript", () => HttpResponse.json(transcript)),
    http.post("/api/runs/:run_id/cancel", () => HttpResponse.json({ canceled: true })),
    http.post("/api/tasks/:slug/runs", () => HttpResponse.json({ run: startRun }, { status: 201 })),
  ];
}

export const handlers = createRunHandlers();
