/** Read the whole skill, file by file, and decide here. */
import { Check, FileText } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  CodeBlock,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Markdown,
  Pill,
} from "@compozy/ui";

import { SkillFileTree } from "./skill-file-tree";
import type { SkillFile, SkillProposal } from "./skill-review-card";

type ReadMode = "read" | "source";

/** Read mode strips raw HTML, so comments and tags never reach the screen. */
const HIDDEN_IN_READ = /<!--|<\/?[a-z][^>]*>/i;

function FileView({ file }: { file: SkillFile }) {
  const [mode, setMode] = useState<ReadMode>("read");
  const markdown = file.language === "markdown";
  const hidden = markdown && HIDDEN_IN_READ.test(file.body);
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <div className="flex min-h-8 items-center gap-3">
        <span className="min-w-0 truncate font-mono text-small-body text-muted-foreground">
          {file.path}
        </span>
        {file.automatic ? (
          <Pill size="sm" tone="warning">
            Runs automatically
          </Pill>
        ) : null}
        {markdown ? (
          <div className="ml-auto flex items-center gap-1">
            <Button
              aria-pressed={mode === "read"}
              onClick={() => setMode("read")}
              size="sm"
              type="button"
              variant={mode === "read" ? "secondary" : "quiet"}
            >
              Read
            </Button>
            <Button
              aria-pressed={mode === "source"}
              onClick={() => setMode("source")}
              size="sm"
              type="button"
              variant={mode === "source" ? "secondary" : "quiet"}
            >
              Source
            </Button>
          </div>
        ) : null}
      </div>
      {hidden && mode === "read" ? (
        <Alert variant="warning">
          <AlertDescription>
            This file has text that doesn't show here.{" "}
            <button
              className="rounded-xs underline outline-none focus-visible:shadow-focus-ring"
              onClick={() => setMode("source")}
              type="button"
            >
              Read the source
            </button>{" "}
            before you decide.
          </AlertDescription>
        </Alert>
      ) : null}
      {markdown && mode === "read" ? (
        <Markdown>{file.body}</Markdown>
      ) : (
        <CodeBlock code={file.body} language={file.language} />
      )}
    </div>
  );
}

export function SkillViewerDialog({
  skill,
  open,
  onOpenChange,
  initialPath,
  stale = false,
  onApprove,
  onReject,
}: {
  skill: SkillProposal;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialPath?: string;
  stale?: boolean;
  onApprove: () => void;
  onReject: () => void;
}) {
  const [selected, setSelected] = useState(initialPath ?? "SKILL.md");
  const file = skill.files.find(item => item.path === selected) ?? skill.files[0]!;
  const count = skill.files.length;
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("xl", { fill: true })}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`${count} ${count === 1 ? "file" : "files"} · Proposed by Triage Lead`}
          icon={FileText}
          onClose={() => onOpenChange(false)}
          title={skill.name}
        />
        <EntityDialogBody className="grid grid-cols-[15rem_minmax(0,1fr)] overflow-hidden p-0 max-[760px]:grid-cols-[minmax(0,1fr)]">
          <nav
            aria-label="Skill files"
            className="min-h-0 overflow-y-auto border-r border-line bg-canvas p-2 max-[760px]:max-h-48 max-[760px]:border-r-0 max-[760px]:border-b"
          >
            <SkillFileTree files={skill.files} onSelect={setSelected} selected={file.path} />
          </nav>
          <div className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto px-6 pt-4 pb-6">
            {stale ? (
              <Alert variant="warning">
                <AlertDescription>
                  This is the latest version. It changed after you first opened it.
                </AlertDescription>
              </Alert>
            ) : null}
            <FileView file={file} key={file.path} />
          </div>
        </EntityDialogBody>
        <EntityDialogFooter
          cancelLabel="Reject"
          onCancel={onReject}
          onPrimary={onApprove}
          primaryIcon={Check}
          primaryLabel="Approve"
        />
      </DialogContent>
    </Dialog>
  );
}
