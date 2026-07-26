import { toast } from "sonner";

type TaskRecordKind = "Task" | "Run";

/** Copies a task-domain identifier with one consistent clipboard/toast policy. */
export async function copyTaskRecordId(id: string, kind: TaskRecordKind): Promise<void> {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (!clipboard) {
    toast.error(`Couldn't copy the ${kind.toLowerCase()} id`);
    return;
  }

  try {
    await clipboard.writeText(id);
    toast.success(`${kind} id copied.`);
  } catch {
    toast.error(`Couldn't copy the ${kind.toLowerCase()} id`);
  }
}
