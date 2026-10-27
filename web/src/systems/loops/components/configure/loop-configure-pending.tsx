import {
  Alert,
  AlertActions,
  AlertDescription,
  EntityDialogBody,
  FormSection,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { LoopConfigureFooter, LoopConfigureHeader } from "./loop-configure-chrome";
import { LoopConfigureStrategy } from "./loop-configure-strategy";
import { LoopConfigureSwitchRowSkeleton } from "./loop-configure-switch-row";
import { LoopWorktreeSectionSkeleton } from "./loop-worktree-section";

const noop = () => undefined;

interface LoopConfigurePendingProps {
  /** The Loop's name, known from the route before any read answers. */
  name: string;
  onClose: () => void;
}

/**
 * The configure dialog's content before the Loop and its settings have answered. The header and
 * footer are the real ones (the footer's actions wait), the section headings and strategy cards
 * are static and real, and the fields are placeholders. It renders inside the same `Dialog` the
 * loaded form does, so the dialog does not close and reopen when the settings arrive.
 */
export function LoopConfigurePending({ name, onClose }: LoopConfigurePendingProps) {
  return (
    <>
      <LoopConfigureHeader name={name} onClose={onClose} />
      <EntityDialogBody className="flex flex-col" data-testid="loop-configure-loading">
        <SkeletonRegion className="flex flex-col" label="Loading settings">
          <Alert role="none" variant="neutral">
            <AlertDescription>Steps, inputs, and the goal stay fixed here.</AlertDescription>
            <AlertActions>
              <Skeleton className="h-button-sm w-44" />
            </AlertActions>
          </Alert>
          <FormSection title="Checks">
            <div className="flex flex-col overflow-hidden rounded-lg bg-sunken">
              {placeholderSlots("check", 2).map(slot => (
                <LoopConfigureSwitchRowSkeleton key={slot.id} />
              ))}
            </div>
          </FormSection>
          <FormSection title="Your approval">
            <div className="overflow-hidden rounded-lg bg-sunken">
              <LoopConfigureSwitchRowSkeleton description={false} title="Ask me before finishing" />
            </div>
          </FormSection>
          <FormSection title="If a round fails">
            <LoopConfigureStrategy disabled onChange={noop} value={null} />
          </FormSection>
          <LoopWorktreeSectionSkeleton />
          <FormSection title="Limits">
            <div className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
              {placeholderSlots("limit", 6).map(slot => (
                <div className="flex flex-col gap-1.5" key={slot.id}>
                  <SkeletonText className="w-28 text-form-label" />
                  <Skeleton className="h-input w-full" />
                </div>
              ))}
            </div>
          </FormSection>
        </SkeletonRegion>
      </EntityDialogBody>
      <LoopConfigureFooter onCancel={onClose} />
    </>
  );
}
