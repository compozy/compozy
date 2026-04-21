import { http, HttpResponse, type HttpHandler } from "msw";

import type { ReviewDetailPayload, ReviewIssue, ReviewSummary, Run } from "../types";
import {
  latestReviewFixture,
  reviewDetailFixture,
  reviewDispatchedRunFixture,
  reviewIssuesFixture,
} from "./fixtures";

export interface ReviewHandlerOptions {
  review?: ReviewSummary;
  issues?: ReviewIssue[];
  detail?: ReviewDetailPayload;
  dispatchedRun?: Run;
}

export function createReviewHandlers(options: ReviewHandlerOptions = {}): HttpHandler[] {
  const review = options.review ?? latestReviewFixture;
  const issues = options.issues ?? reviewIssuesFixture;
  const detail = options.detail ?? reviewDetailFixture;
  const dispatchedRun = options.dispatchedRun ?? reviewDispatchedRunFixture;

  return [
    http.get("/api/reviews/:slug", () => HttpResponse.json({ review })),
    http.get("/api/reviews/:slug/rounds/:round/issues", () => HttpResponse.json({ issues })),
    http.get("/api/reviews/:slug/rounds/:round/issues/:issue_id", () =>
      HttpResponse.json({ review: detail })
    ),
    http.post("/api/reviews/:slug/rounds/:round/runs", () =>
      HttpResponse.json({ run: dispatchedRun }, { status: 201 })
    ),
  ];
}

export const handlers = createReviewHandlers();
