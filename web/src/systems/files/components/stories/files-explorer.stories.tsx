import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { CodeHighlightPoolProvider } from "@compozy/ui";
import { createCodeHighlightWorker } from "@compozy/ui/lib/code-highlight-worker";

import { storybookMswParameters } from "@/storybook/msw";

import type { FilesRootRef } from "../../adapters/files-api";
import {
  fileViewerTarget,
  outsideViewerTarget,
  type FileViewerTarget,
} from "../../lib/file-viewer-target";
import {
  sessionFilesHandlers,
  sessionFilesRootUnavailableHandlers,
} from "../../mocks/files-handlers";
import { FilesExplorer } from "../files-explorer";

const ROOT: FilesRootRef = { kind: "session", workspaceId: "ws_alpha", sessionId: "sess-files" };

function Explorer({ initial }: { initial: FileViewerTarget | null }) {
  const [target, setTarget] = useState(initial);
  return (
    <CodeHighlightPoolProvider workerFactory={createCodeHighlightWorker}>
      <div className="flex h-[620px] w-full flex-col border border-line bg-canvas">
        <FilesExplorer
          root={ROOT}
          target={target}
          onOpenFile={path => setTarget(fileViewerTarget(path))}
        />
      </div>
    </CodeHighlightPoolProvider>
  );
}

/**
 * The project files explorer (`_uiux.md` S3/S4): the lazy tree and the read-only viewer with
 * every truthful state. Session Review panel's Files tab and the Files app compose it.
 */
const meta = {
  title: "systems/files/components/FilesExplorer",
  component: Explorer,
  parameters: { layout: "fullscreen", ...storybookMswParameters({ files: sessionFilesHandlers }) },
  args: { initial: null },
} satisfies Meta<typeof Explorer>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NoSelection: Story = {};

export const TextAtLine: Story = { args: { initial: fileViewerTarget("web/src/main.tsx", 12) } };

export const LinePastEnd: Story = {
  args: { initial: fileViewerTarget("web/src/main.tsx", 420) },
};

export const Truncated: Story = { args: { initial: fileViewerTarget("README.md") } };

export const SensitiveGate: Story = { args: { initial: fileViewerTarget(".env") } };

export const Image: Story = { args: { initial: fileViewerTarget("docs/logo.png") } };

export const Binary: Story = { args: { initial: fileViewerTarget("docs/diagram.bin") } };

export const NotFound: Story = { args: { initial: fileViewerTarget("internal/gone.go") } };

export const OutsideProject: Story = { args: { initial: outsideViewerTarget("/tmp/report.md") } };

export const RootUnavailable: Story = {
  parameters: storybookMswParameters({ files: sessionFilesRootUnavailableHandlers }),
};

export const TextAtLineLight: Story = {
  args: { initial: fileViewerTarget("web/src/main.tsx", 12) },
  globals: { theme: "light" },
};

export const SensitiveGateLight: Story = {
  args: { initial: fileViewerTarget(".env") },
  globals: { theme: "light" },
};
