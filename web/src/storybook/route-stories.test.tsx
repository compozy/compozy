import { cleanup, screen } from "@testing-library/react";
import { composeStories, setProjectAnnotations } from "@storybook/react-vite";
import { afterEach, describe, expect, it } from "vitest";

import * as preview from "../../.storybook/preview";
import * as dashboardStories from "@/routes/_app/stories/-dashboard.stories";
import * as memoryIndexStories from "@/routes/_app/stories/-memory.stories";
import * as memoryStories from "@/routes/_app/stories/-memory.$slug.stories";
import * as reviewDetailStories from "@/routes/_app/stories/-reviews.$slug.$round.$issueId.stories";
import * as reviewStories from "@/routes/_app/stories/-reviews.stories";
import * as runDetailStories from "@/routes/_app/stories/-runs.$runId.stories";
import * as runStories from "@/routes/_app/stories/-runs.stories";
import * as specStories from "@/routes/_app/stories/-workflows.$slug.spec.stories";
import * as taskBoardStories from "@/routes/_app/stories/-workflows.$slug.tasks.stories";
import * as taskDetailStories from "@/routes/_app/stories/-workflows.$slug.tasks.$taskId.stories";
import * as workflowStories from "@/routes/_app/stories/-workflows.stories";

setProjectAnnotations(preview);

const { Degraded: DashboardDegraded } = composeStories(dashboardStories);
const { Loading: DashboardLoading } = composeStories(dashboardStories);
const { Empty: MemoryIndexEmpty } = composeStories(memoryIndexStories);
const { Empty: WorkflowsEmpty } = composeStories(workflowStories);
const { Error: WorkflowsError } = composeStories(workflowStories);
const { Empty: TaskBoardEmpty } = composeStories(taskBoardStories);
const { Error: TaskDetailError } = composeStories(taskDetailStories);
const { Empty: RunsEmpty } = composeStories(runStories);
const { Overflowed: RunOverflowed } = composeStories(runDetailStories);
const { PartialIssuesError: ReviewsPartialIssuesError } = composeStories(reviewStories);
const { Success: ReviewDetailSuccess } = composeStories(reviewDetailStories);
const { PartialDocuments: SpecPartialDocuments } = composeStories(specStories);
const { DocumentError: MemoryDocumentError } = composeStories(memoryStories);

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
});

describe("portable route stories", () => {
  it("renders the degraded dashboard state", async () => {
    await DashboardDegraded.run();

    expect(await screen.findByTestId("dashboard-view")).toBeInTheDocument();
    expect(screen.getAllByText("degraded").length).toBeGreaterThan(0);
  });

  it("renders the dashboard loading state", async () => {
    await DashboardLoading.run();

    expect(await screen.findByTestId("dashboard-loading")).toBeInTheDocument();
  });

  it("renders the empty workflow inventory state", async () => {
    await WorkflowsEmpty.run();

    expect(await screen.findByTestId("workflow-inventory-empty")).toBeInTheDocument();
  });

  it("renders the workflow inventory error state", async () => {
    await WorkflowsError.run();

    expect(await screen.findByTestId("workflow-inventory-load-error")).toBeInTheDocument();
  });

  it("renders the empty workflow task-board state", async () => {
    await TaskBoardEmpty.run();

    expect(await screen.findByTestId("task-board-empty")).toBeInTheDocument();
  });

  it("renders the task-detail error branch", async () => {
    await TaskDetailError.run();

    expect(await screen.findByTestId("task-detail-load-error")).toBeInTheDocument();
  });

  it("renders the empty run inventory state", async () => {
    await RunsEmpty.run();

    expect(await screen.findByTestId("runs-list-empty")).toBeInTheDocument();
  });

  it("renders the degraded run overflow state", async () => {
    await RunOverflowed.run();

    expect(await screen.findByTestId("run-detail-stream-overflow")).toBeInTheDocument();
  });

  it("renders the partial review index state when issue loading fails", async () => {
    await ReviewsPartialIssuesError.run();

    expect(await screen.findByTestId("reviews-index-card-issues-error-alpha")).toBeInTheDocument();
  });

  it("renders the review issue detail success state", async () => {
    await ReviewDetailSuccess.run();

    expect(await screen.findByTestId("review-detail-view")).toBeInTheDocument();
  });

  it("renders the partial spec state and falls back to techspec content", async () => {
    await SpecPartialDocuments.run();

    expect(await screen.findByTestId("workflow-spec-techspec-body")).toBeInTheDocument();
  });

  it("renders the memory detail document-error state", async () => {
    await MemoryDocumentError.run();

    expect(await screen.findByTestId("workflow-memory-document-error")).toBeInTheDocument();
  });

  it("renders the empty memory index state", async () => {
    await MemoryIndexEmpty.run();

    expect(await screen.findByTestId("memory-index-empty")).toBeInTheDocument();
  });
});
