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

import { alignLines, splitLines, ToolDiffBody } from "@/systems/session";

import { hasInvisibleCharacters, skillMarkdownRead } from "../../lib/agent-skill-markdown-read";
import {
  SKILL_DEFINITION_FILE,
  skillReviewChangeCounts,
  type SkillReviewFile,
} from "../../lib/agent-skill-review";
import type { AgentSkillLessonPayload } from "../../types";
import { AgentSkillFileTree } from "./agent-skill-file-tree";
import { AgentSkillLesson } from "./agent-skill-lesson";

type ReadMode = "read" | "source" | "diff";

const MODE_LABEL: Record<ReadMode, string> = { read: "Read", source: "Source", diff: "Changes" };

const CHANGE_LABEL = { added: "Added", removed: "Removed", modified: "Modified" } as const;

const NO_EVIDENCE_SESSIONS: readonly string[] = [];

function ReadToggle({
  modes,
  mode,
  onMode,
}: {
  modes: readonly ReadMode[];
  mode: ReadMode;
  onMode: (mode: ReadMode) => void;
}) {
  return (
    <div className="ml-auto flex items-center gap-1">
      {modes.map(value => (
        <Button
          aria-pressed={mode === value}
          key={value}
          onClick={() => onMode(value)}
          size="sm"
          type="button"
          variant={mode === value ? "secondary" : "quiet"}
        >
          {MODE_LABEL[value]}
        </Button>
      ))}
    </div>
  );
}

/** Source shows every byte, but some of them draw nothing: say so, on any file. */
function InvisibleCharactersNotice() {
  return (
    <Alert data-testid="skill-review-invisible-characters" variant="warning">
      <AlertDescription>
        This file has characters that don't show, such as zero-width or text-direction marks. They
        can change what it says or does.
      </AlertDescription>
    </Alert>
  );
}

/** Approve or Revert refused because the skill changed while it was being replaced: the op to resolve. */
export function SkillInstallConflictAlert({ op }: { op: string }) {
  return (
    <Alert data-testid="skill-review-install-conflict" variant="warning">
      <AlertDescription>
        This skill changed while it was being replaced. Resolve it with{" "}
        <code className="font-mono">
          compozy agent skill install-conflict resolve {op || "<op>"}
        </code>
        .
      </AlertDescription>
    </Alert>
  );
}

/** An installed file that isn't text: name and size only. */
function BinaryFileView({ file }: { file: SkillReviewFile }) {
  return (
    <div className="flex min-w-0 flex-col gap-3" data-testid="skill-review-binary">
      <span className="min-w-0 truncate font-mono text-small-body text-muted-foreground">
        {file.path}
      </span>
      <p className="text-small-body text-muted-foreground">
        Not a text file · {file.binarySize ?? 0} bytes
        {file.change === "removed" ? " · Removed" : " · Kept as it is"}
      </p>
    </div>
  );
}

/** The body of a text file in the chosen mode: the diff of an edit, rendered Markdown, or source. */
function FileBody({
  file,
  mode,
  read,
}: {
  file: SkillReviewFile;
  mode: ReadMode;
  read: ReturnType<typeof skillMarkdownRead> | null;
}) {
  if (mode === "diff") {
    return (
      <ToolDiffBody
        lines={alignLines(splitLines(file.baseBody ?? ""), splitLines(file.body))}
        path={file.path}
        testId="skill-review-diff"
      />
    );
  }
  if (read && mode === "read") return <Markdown>{read.text}</Markdown>;
  return <CodeBlock code={file.body} language={file.language} />;
}

/** The modes a text file offers, first one the default: Changes for an edit, Read for Markdown, Source. */
function fileReadModes(file: SkillReviewFile): ReadMode[] {
  const modes: ReadMode[] = [];
  if (file.baseBody !== undefined) modes.push("diff");
  if (file.language === "markdown") modes.push("read");
  modes.push("source");
  return modes;
}

/** The file's change and "Runs automatically" pills. */
function FilePills({ file }: { file: SkillReviewFile }) {
  const change = file.change && file.change !== "unchanged" ? CHANGE_LABEL[file.change] : null;
  return (
    <>
      {change ? <Pill size="sm">{change}</Pill> : null}
      {file.automatic ? (
        <Pill size="sm" tone="warning">
          Runs automatically
        </Pill>
      ) : null}
    </>
  );
}

/** Rendered Markdown hides some of the file's text: point at the source. */
function HiddenTextNotice({ onReadSource }: { onReadSource: () => void }) {
  return (
    <Alert data-testid="skill-review-hidden-text" variant="warning">
      <AlertDescription>
        This file has text that doesn't show here.{" "}
        <button
          className="rounded-xs underline outline-none focus-visible:shadow-focus-ring"
          onClick={onReadSource}
          type="button"
        >
          Read the source
        </button>{" "}
        before you decide.
      </AlertDescription>
    </Alert>
  );
}

function FileView({ file }: { file: SkillReviewFile }) {
  // Only the operator's choice is state; the default mode follows the file (the first one offered).
  const [chosen, setChosen] = useState<ReadMode | null>(null);
  if (file.binarySize !== undefined) return <BinaryFileView file={file} />;
  const modes = fileReadModes(file);
  const mode = chosen !== null && modes.includes(chosen) ? chosen : (modes[0] ?? "source");
  const read = file.language === "markdown" ? skillMarkdownRead(file.body) : null;
  const readHides = read?.hidesText === true && mode === "read";
  const invisible = !readHides && hasInvisibleCharacters(file.body);
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <div className="flex min-h-8 items-center gap-3">
        <span className="min-w-0 truncate font-mono text-small-body text-muted-foreground">
          {file.path}
        </span>
        <FilePills file={file} />
        {modes.length > 1 ? <ReadToggle mode={mode} modes={modes} onMode={setChosen} /> : null}
      </div>
      {invisible ? <InvisibleCharactersNotice /> : null}
      {readHides ? <HiddenTextNotice onReadSource={() => setChosen("source")} /> : null}
      <FileBody file={file} mode={mode} read={read} />
    </div>
  );
}

export function AgentSkillViewerDialog({
  name,
  agentLabel,
  files,
  lesson,
  evidenceSessionIds = NO_EVIDENCE_SESSIONS,
  unavailableEvidenceSessionIds = NO_EVIDENCE_SESSIONS,
  open,
  onOpenChange,
  stale = false,
  decided = false,
  deciding = false,
  approveDisabled = false,
  error,
  installConflictOp = null,
  onApprove,
  onReject,
  initialPath,
  automaticComparison,
}: {
  /** Edit reviews: "Runs automatically" for this version and for the installed skill. */
  automaticComparison?: { proposed: string; installed: string };
  /** The file shown first; SKILL.md when absent. */
  initialPath?: string;
  name: string;
  agentLabel: string;
  files: readonly SkillReviewFile[];
  /** The lesson the agent proposed with; absent on reviews created before lessons. */
  lesson?: AgentSkillLessonPayload | null;
  evidenceSessionIds?: readonly string[];
  /** Evidence sessions the daemon no longer knows. */
  unavailableEvidenceSessionIds?: readonly string[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The contents changed after the operator first read them; this is the latest. */
  stale?: boolean;
  /** Already approved or rejected: read only. */
  decided?: boolean;
  deciding?: boolean;
  /** This version can't be approved; Reject stays available. */
  approveDisabled?: boolean;
  error?: string;
  /** Approve was refused because the skill changed while it was being replaced: the op to resolve. */
  installConflictOp?: string | null;
  onApprove: () => void;
  onReject: () => void;
}) {
  const [selected, setSelected] = useState(initialPath ?? SKILL_DEFINITION_FILE);
  const file = files.find(item => item.path === selected) ?? files[0];
  const count = files.length;
  const changes = skillReviewChangeCounts(files);
  const edit = files.some(item => item.change !== undefined);
  const description = edit
    ? `${changes.modified} modified, ${changes.added} added, ${changes.removed} removed · Proposed by ${agentLabel}`
    : `${count} ${count === 1 ? "file" : "files"} · Proposed by ${agentLabel}`;
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("xl", { fill: true })}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={description}
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
            <AgentSkillLesson
              evidenceSessionIds={evidenceSessionIds}
              lesson={lesson}
              unavailableSessionIds={unavailableEvidenceSessionIds}
            />
            {automaticComparison ? (
              <p
                className="text-small-body text-muted-foreground"
                data-testid="skill-review-automatic-counts"
              >
                Runs automatically: {automaticComparison.proposed} (installed now:{" "}
                {automaticComparison.installed})
              </p>
            ) : null}
            {stale ? (
              <Alert data-testid="skill-review-stale" variant="warning">
                <AlertDescription>
                  This is the latest version. It changed after you first opened it.
                </AlertDescription>
              </Alert>
            ) : null}
            {installConflictOp !== null ? (
              <SkillInstallConflictAlert op={installConflictOp} />
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
            primaryDisabled={approveDisabled}
            isSaving={deciding}
            primaryIcon={Check}
            primaryLabel="Approve"
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
