/** S9: read the whole proposed skill, file by file, and decide here. */
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

import { skillMarkdownRead } from "../../lib/agent-skill-markdown-read";
import { SKILL_DEFINITION_FILE, type SkillReviewFile } from "../../lib/agent-skill-review";
import { AgentSkillFileTree } from "./agent-skill-file-tree";

type ReadMode = "read" | "source";

function ReadToggle({ mode, onMode }: { mode: ReadMode; onMode: (mode: ReadMode) => void }) {
  return (
    <div className="ml-auto flex items-center gap-1">
      {(["read", "source"] as const).map(value => (
        <Button
          aria-pressed={mode === value}
          key={value}
          onClick={() => onMode(value)}
          size="sm"
          type="button"
          variant={mode === value ? "secondary" : "quiet"}
        >
          {value === "read" ? "Read" : "Source"}
        </Button>
      ))}
    </div>
  );
}

function FileView({ file }: { file: SkillReviewFile }) {
  const [mode, setMode] = useState<ReadMode>("read");
  const markdown = file.language === "markdown";
  const read = markdown ? skillMarkdownRead(file.body) : null;
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
        {markdown ? <ReadToggle mode={mode} onMode={setMode} /> : null}
      </div>
      {read?.hidesText && mode === "read" ? (
        <Alert data-testid="skill-review-hidden-text" variant="warning">
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
      {read && mode === "read" ? (
        <Markdown>{read.text}</Markdown>
      ) : (
        <CodeBlock code={file.body} language={file.language} />
      )}
    </div>
  );
}

export function AgentSkillViewerDialog({
  name,
  agentLabel,
  files,
  open,
  onOpenChange,
  stale = false,
  decided = false,
  deciding = false,
  error,
  onApprove,
  onReject,
  initialPath,
}: {
  /** The file shown first; SKILL.md when absent. */
  initialPath?: string;
  name: string;
  agentLabel: string;
  files: readonly SkillReviewFile[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The contents changed after the operator first read them; this is the latest. */
  stale?: boolean;
  /** Already approved or rejected: read only. */
  decided?: boolean;
  deciding?: boolean;
  error?: string;
  onApprove: () => void;
  onReject: () => void;
}) {
  const [selected, setSelected] = useState(initialPath ?? SKILL_DEFINITION_FILE);
  const file = files.find(item => item.path === selected) ?? files[0];
  const count = files.length;
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("xl", { fill: true })}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`${count} ${count === 1 ? "file" : "files"} · Proposed by ${agentLabel}`}
          icon={FileText}
          onClose={() => onOpenChange(false)}
          title={name}
        />
        <EntityDialogBody className="grid grid-cols-[15rem_minmax(0,1fr)] overflow-hidden p-0 max-[760px]:grid-cols-[minmax(0,1fr)]">
          <nav
            aria-label="Skill files"
            className="min-h-0 overflow-y-auto border-r border-line bg-canvas p-2 max-[760px]:max-h-48 max-[760px]:border-r-0 max-[760px]:border-b"
          >
            <AgentSkillFileTree files={files} onSelect={setSelected} selected={file?.path ?? ""} />
          </nav>
          <div className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto px-6 pt-4 pb-6">
            {stale ? (
              <Alert data-testid="skill-review-stale" variant="warning">
                <AlertDescription>
                  This is the latest version. It changed after you first opened it.
                </AlertDescription>
              </Alert>
            ) : null}
            {error ? (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            {file ? <FileView file={file} key={file.path} /> : null}
          </div>
        </EntityDialogBody>
        {decided ? null : (
          <EntityDialogFooter
            cancelLabel="Reject"
            onCancel={onReject}
            onPrimary={onApprove}
            cancelDisabled={deciding}
            isSaving={deciding}
            primaryIcon={Check}
            primaryLabel="Approve"
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
