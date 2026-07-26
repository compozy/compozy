import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Empty,
  Kbd,
  Pill,
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
  Section,
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  toast,
} from "@agh/ui";
import { BellIcon, GitBranchIcon, InfoIcon, Layers3Icon, PlayIcon } from "lucide-react";

import { SectionLink } from "./design-system-showcase-section-link";
import { sectionById } from "./design-system-showcase-sections";

export function FeedbackSection() {
  return (
    <Section
      id="feedback"
      data-testid="section-feedback"
      label={
        <SectionLink section={sectionById("feedback")}>
          Feedback (Alert, Empty, Toaster)
        </SectionLink>
      }
      right={
        <Pill mono tone="warning">
          state
        </Pill>
      }
    >
      <div className="grid gap-3 pt-4 md:grid-cols-2">
        <Alert>
          <InfoIcon />
          <AlertTitle>Scheduled maintenance</AlertTitle>
          <AlertDescription>
            Runtime will reload at 04:00 UTC to pick up skill index updates.
          </AlertDescription>
        </Alert>
        <Alert variant="danger">
          <InfoIcon />
          <AlertTitle>Connection lost</AlertTitle>
          <AlertDescription>
            Reconnecting to the daemon. Check the footer indicator for status.
          </AlertDescription>
        </Alert>
      </div>
      <div className="pt-4">
        <Empty
          icon={Layers3Icon}
          title="Ready for your first session"
          description="Pick an agent above or hit ⌘K to spawn a new run. Every session is replayable by default."
          action={
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                onClick={() =>
                  toast("Toast fired", { description: "Sonner works out of the box." })
                }
              >
                Fire toast
              </Button>
              <Button size="sm" variant="outline">
                Read the docs
              </Button>
            </div>
          }
        />
      </div>
    </Section>
  );
}

export function OverlaysSection() {
  return (
    <Section
      id="overlays"
      data-testid="section-overlays"
      label={
        <SectionLink section={sectionById("overlays")}>
          Dialog · Sheet · Popover · Tooltip
        </SectionLink>
      }
      right={
        <Pill mono tone="accent">
          motion
        </Pill>
      }
    >
      <div className="flex flex-wrap items-center gap-3 pt-4">
        <Dialog>
          <DialogTrigger
            render={<Button variant="outline" data-testid="showcase-dialog-trigger" />}
          >
            Open dialog
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delete session?</DialogTitle>
              <DialogDescription>
                Deleting removes the replay events. This cannot be undone.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
              <Button variant="destructive">Delete</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Sheet>
          <SheetTrigger render={<Button variant="outline" data-testid="showcase-sheet-trigger" />}>
            Open sheet
          </SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Session settings</SheetTitle>
              <SheetDescription>
                Configure per-session defaults without leaving the page.
              </SheetDescription>
            </SheetHeader>
            <SheetFooter>
              <SheetClose render={<Button variant="outline" />}>Close</SheetClose>
              <Button>Save changes</Button>
            </SheetFooter>
          </SheetContent>
        </Sheet>

        <Popover>
          <PopoverTrigger
            render={<Button variant="outline" data-testid="showcase-popover-trigger" />}
          >
            Open popover
          </PopoverTrigger>
          <PopoverContent>
            <PopoverHeader>
              <PopoverTitle>Quick tip</PopoverTitle>
              <PopoverDescription>
                Press <Kbd>⌘</Kbd> + <Kbd>K</Kbd> to open the command palette.
              </PopoverDescription>
            </PopoverHeader>
          </PopoverContent>
        </Popover>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button variant="outline" data-testid="showcase-tooltip-trigger">
                Hover me
              </Button>
            }
          />
          <TooltipContent>Tooltip body copy</TooltipContent>
        </Tooltip>

        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="outline" data-testid="showcase-menu-trigger" />}
          >
            Dropdown
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem>
              <PlayIcon className="size-3" /> Start run
            </DropdownMenuItem>
            <DropdownMenuItem>
              <GitBranchIcon className="size-3" /> Fork session
            </DropdownMenuItem>
            <DropdownMenuItem>
              <BellIcon className="size-3" /> Notify on completion
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive">Delete</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div className="flex flex-col gap-4 pt-6">
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="events">Events</TabsTrigger>
            <TabsTrigger value="artifacts">Artifacts</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">
            <p className="text-sm text-muted">
              Tabs host section switches: Base UI driven, motion-free.
            </p>
          </TabsContent>
          <TabsContent value="events">
            <p className="text-sm text-muted">
              Replayable event timeline lives under this tab in production.
            </p>
          </TabsContent>
          <TabsContent value="artifacts">
            <p className="text-sm text-muted">Generated files with their provenance chain.</p>
          </TabsContent>
        </Tabs>
        <Accordion defaultValue={["item-1"]}>
          <AccordionItem value="item-1">
            <AccordionTrigger>Can I rewind a session?</AccordionTrigger>
            <AccordionContent>
              Yes, every session is fully replayable from its event store.
            </AccordionContent>
          </AccordionItem>
          <AccordionItem value="item-2">
            <AccordionTrigger>Does it work offline?</AccordionTrigger>
            <AccordionContent>
              The runtime is local-first. The network protocol only activates for peer flows.
            </AccordionContent>
          </AccordionItem>
        </Accordion>
        <Collapsible>
          <CollapsibleTrigger
            render={<Button variant="ghost" data-testid="showcase-collapsible-trigger" />}
          >
            Toggle diagnostics
          </CollapsibleTrigger>
          <CollapsibleContent>
            <p className="text-sm text-muted">Collapsed content reveals with a CSS animation.</p>
          </CollapsibleContent>
        </Collapsible>
      </div>
    </Section>
  );
}
