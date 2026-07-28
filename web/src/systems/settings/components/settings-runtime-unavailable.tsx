import { AlertTriangle } from "lucide-react";

export function SettingsRuntimeUnavailable({
  slug,
  description = "Live runtime data is unavailable. Saved configuration remains editable.",
}: {
  slug: string;
  description?: string;
}) {
  return (
    <div
      className="flex items-start gap-2.5 rounded-md bg-warning-tint px-3.5 py-3 text-form-label leading-normal text-muted"
      data-testid={`settings-page-${slug}-runtime-unavailable`}
      role="status"
    >
      <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
      <div className="min-w-0">
        <span className="block text-ws-name font-medium text-fg-strong">
          Runtime data unavailable
        </span>
        {description}
      </div>
    </div>
  );
}
