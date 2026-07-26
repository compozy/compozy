export * from "./primitives";

// Topbar shell and route-slot publication API.
export {
  Topbar,
  TopbarOverflowIcon,
  TopbarSlotProvider,
  useTopbarSlot,
  useTopbarSlotValue,
  type TopbarCrumb,
  type TopbarProps,
  type TopbarSlotProviderProps,
  type TopbarSlotValue,
} from "./components/custom/topbar";
export { RouteNav } from "./components/custom/route-nav";

// Promoted from `web/src/systems/network/components/`.
export { KindChip, type KindChipProps } from "./components/custom/kind-chip";
export { RightRail, type RightRailMode, type RightRailProps } from "./components/custom/right-rail";

// Net-new shared-kit composites (P3).
export { LaneTabs, type LaneTabsItem, type LaneTabsProps } from "./components/custom/lane-tabs";
export {
  DayAreaChart,
  type DayAreaChartDatum,
  type DayAreaChartProps,
} from "./components/custom/day-area-chart";
export {
  DayStackedBars,
  type DayStackedBarsDatum,
  type DayStackedBarsProps,
  type DayStackedBarsSeries,
} from "./components/custom/day-stacked-bars";
export { Panel, type PanelProps } from "./components/custom/panel";
export { Sparkline, type SparklineProps } from "./components/custom/sparkline";
export { IntensityMeter, type IntensityMeterProps } from "./components/custom/intensity-meter";
export {
  ContextBox,
  type ContextBoxEntry,
  type ContextBoxProps,
} from "./components/custom/context-box";
export { JsonViewer, type JsonViewerProps } from "./components/custom/json-viewer";
export { EditorFooter, type EditorFooterProps } from "./components/custom/editor-footer";
export {
  StatusBreakdown,
  type StatusBreakdownItem,
  type StatusBreakdownProps,
} from "./components/custom/status-breakdown";
export { MetadataTile, type MetadataTileProps } from "./components/custom/metadata-tile";
export { FormSection, type FormSectionProps } from "./components/custom/form-section";
export { HelpTip, type HelpTipProps } from "./components/custom/help-tip";
export { Icon, type IconProps, type IconSize } from "./components/icon";
export { MonoId, type MonoIdProps, type MonoIdSize } from "./components/custom/mono-id";
export { Time, type TimeMode, type TimeProps } from "./components/custom/time";
export {
  StatusDot,
  type StatusDotProps,
  type StatusDotSize,
  type StatusDotTone,
  type StatusDotVariant,
} from "./components/custom/status-dot";
export {
  formatAbsoluteTime,
  formatDuration,
  formatRelativeTime,
  FORMAT_TIME_FALLBACK,
} from "./lib/format-time";
export {
  AGH_CODE_DEFAULT_THEME,
  AGH_CODE_SUPPORTED_LANGUAGES,
  AGH_CODE_THEMES,
  normalizeAghCodeLanguage,
  resolveAghCodeThemeName,
  type AghCodeLanguage,
  type AghCodeThemeName,
  type CodeBlockResolvedTheme,
  type CodeBlockThemeMode,
} from "./lib/code-theme";
export {
  AGENT_SLOT_COUNT,
  HUMAN_SLOT_COUNT,
  SYSTEM_SLOT_COUNT,
  colorsFor,
  seed,
  type OwnerColors,
  type OwnerKind,
} from "./lib/owner-palette";
export {
  WIDTH_DETAIL_INSPECTOR_INLINE,
  WIDTH_MESSAGE_BUBBLE_MAX,
  WIDTH_RIGHT_RAIL_DEFAULT,
  WIDTH_TABLE_CELL_LG,
} from "./lib/layout-widths";

// Entity-editor modal shell (modals-redesign F1-F7).
export {
  dialogShellClass,
  type DialogShellOptions,
  type DialogShellSize,
} from "./lib/dialog-shell";
export {
  EntityDialogHeader,
  type EntityDialogHeaderProps,
} from "./components/custom/entity-dialog-header";
export {
  EntityDialogFooter,
  type EntityDialogFooterProps,
} from "./components/custom/entity-dialog-footer";
export {
  EntityDialogBody,
  type EntityDialogBodyProps,
  type EntityDialogBodyVariant,
} from "./components/custom/entity-dialog-body";
export {
  EntityDialogToolbar,
  type EntityDialogToolbarProps,
} from "./components/custom/entity-dialog-toolbar";
export {
  EntityModeToolbar,
  type EntityMode,
  type EntityModeToolbarProps,
} from "./components/custom/entity-mode-toolbar";
export {
  SecretField,
  type SecretFieldBinding,
  type SecretFieldMode,
  type SecretFieldProps,
  type SecretFieldSource,
  type SecretFieldSourceCreate,
  type SecretFieldState,
} from "./components/custom/secret-field";
export {
  ImmutableIdentity,
  type ImmutableIdentityProps,
  type ImmutableIdentityRow,
} from "./components/custom/immutable-identity";
export { RequiredMark, type RequiredMarkProps } from "./components/custom/required-mark";

export { Markdown, STREAMDOWN_SAFE_CONFIG, type MarkdownProps } from "./components/custom/markdown";
export { DescriptionCard, type DescriptionCardProps } from "./components/custom/description-card";
export { StreamMarkdown, type StreamMarkdownProps } from "./components/custom/stream-markdown";
export {
  RUN_STATUS_LABEL,
  RUN_STATUS_TONE,
  RunCard,
  type RunCardProps,
  type RunCardStatus,
  type RunCardWarning,
  type RunCardWarningTone,
} from "./components/custom/run-card";
export {
  OwnerAvatar,
  type OwnerAvatarProps,
  type OwnerAvatarSize,
} from "./components/custom/owner-avatar";
export {
  RestartBanner,
  type RestartBannerProps,
  type RestartBannerTone,
} from "./components/custom/restart-banner";
export {
  PageActionsTopbarSlot,
  type PageActionsTopbarSlotProps,
} from "./components/custom/page-actions-topbar-slot";
export {
  StatusLine,
  type StatusLineItem,
  type StatusLineProps,
} from "./components/custom/status-line";
export {
  DETAIL_INSPECTOR_INLINE_BREAKPOINT,
  DETAIL_INSPECTOR_INLINE_WIDTH,
  DetailInspector,
  type DetailInspectorProps,
  type DetailInspectorTab,
} from "./components/custom/detail-inspector";
export {
  QueueHealthSparkline,
  type QueueHealthSparklineBucket,
  type QueueHealthSparklineProps,
} from "./components/custom/queue-health-sparkline";
export { RadioCard, type RadioCardProps } from "./components/custom/radio-card";
export {
  ActionResultBanner,
  type ActionResultBannerProps,
  type ActionResultBannerTone,
} from "./components/custom/action-result-banner";
export {
  StackedProgress,
  type StackedProgressProps,
  type StackedProgressSegment,
} from "./components/custom/stacked-progress";
export { Timeline, type TimelineProps } from "./components/custom/timeline";
export { TimelineEvent, type TimelineEventProps } from "./components/custom/timeline-event";
export {
  PriorityBars,
  type PriorityBarsProps,
  type PriorityLevel,
} from "./components/custom/priority-bars";
export {
  OperationalLinksRow,
  type OperationalLink,
  type OperationalLinksRowProps,
} from "./components/custom/operational-links-row";
export {
  WireCard,
  WireCardHead,
  WireCardBody,
  WireCardFoot,
  type WireCardProps,
} from "./components/custom/wire-card";
export { TypingDots, type TypingDotsProps } from "./components/custom/typing-dots";
export {
  CodeBlock,
  CopyIconButton,
  type CodeBlockDensity,
  type CodeBlockHighlightState,
  type CodeBlockProps,
  type CodeBlockTone,
  type CopyIconButtonProps,
} from "./components/custom/code-block";
export {
  BlockLoading,
  type BlockLoadingProps,
  type BlockLoadingSize,
  type BlockLoadingSurface,
} from "./components/custom/block-loading";
export {
  DataSurface,
  DataSurfaceContent,
  DataSurfaceEmpty,
  DataSurfaceError,
  DataSurfaceLoading,
  type DataSurfaceContentProps,
  type DataSurfaceEmptyProps,
  type DataSurfaceErrorProps,
  type DataSurfaceLoadingProps,
  type DataSurfaceProps,
  type DataSurfaceState,
} from "./components/custom/data-surface";
export { resolveDataSurfaceState } from "./components/custom/data-surface-state";
export { LiveBadge, type LiveBadgeProps } from "./components/custom/live-badge";
export { PropertyRow, type PropertyRowProps } from "./components/custom/property-row";
export {
  ConnectionIndicator,
  type ConnectionIndicatorDotProps,
  type ConnectionIndicatorLabelProps,
  type ConnectionIndicatorProps,
  type ConnectionStatus,
  type ConnectionVariant,
} from "./components/custom/connection-indicator";
export {
  StatusCard,
  type StatusCardActionProps,
  type StatusCardBodyProps,
  type StatusCardFooterProps,
  type StatusCardHeaderProps,
  type StatusCardProps,
  type StatusCardTone,
} from "./components/custom/status-card";
export {
  ConfirmDialog,
  type ConfirmDialogNoteTone,
  type ConfirmDialogProps,
  type ConfirmDialogTone,
} from "./components/custom/confirm-dialog";
export {
  CatalogCard,
  type CatalogCardActionsProps,
  type CatalogCardDescriptionProps,
  type CatalogCardLogoProps,
  type CatalogCardLogoSize,
  type CatalogCardMetaProps,
  type CatalogCardProps,
  type CatalogCardTitleProps,
  type CatalogCardTone,
} from "./components/custom/catalog-card";
export {
  ListingRow,
  type ListingRowDescriptionProps,
  type ListingRowIconProps,
  type ListingRowLinkProps,
  type ListingRowMainProps,
  type ListingRowMetaProps,
  type ListingRowNameProps,
  type ListingRowProps,
  type ListingRowSlugProps,
  type ListingRowStatProps,
  type ListingRowTitleProps,
  type ListingRowTrailProps,
} from "./components/custom/listing-row";
export {
  ListingToolbar,
  type ListingToolbarFiltersProps,
  type ListingToolbarLeadingProps,
  type ListingToolbarProps,
  type ListingToolbarSearchProps,
  type ListingToolbarTrailingProps,
  type ListingToolbarViewToggleProps,
  type ListingViewMode,
} from "./components/custom/listing-toolbar";
export { ListingPage, type ListingPageProps } from "./components/custom/listing-page";
export {
  ListGroup,
  ListGroupHeader,
  ListGroupItems,
  ListGroupRoot,
  type ListGroupHeaderProps,
  type ListGroupItemsProps,
  type ListGroupProps,
} from "./components/custom/list-group";
export {
  CommandSelect,
  CommandSelectChip,
  CommandSelectChipStrip,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  type CommandSelectChipProps,
  type CommandSelectChipStripProps,
  type CommandSelectGroupProps,
  type CommandSelectProps,
  type CommandSelectShellProps,
  type CommandSelectTriggerProps,
} from "./components/custom/command-select";
export {
  MetadataList,
  MetadataListRoot,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  type MetadataListProps,
  type MetadataListRowProps,
  type MetadataListTermProps,
  type MetadataListValueProps,
} from "./components/custom/metadata-list";
export {
  LinkedRecordTable,
  LinkedRecordTableBody,
  LinkedRecordTableCell,
  LinkedRecordTableOpenCell,
  LinkedRecordTableRoot,
  LinkedRecordTableRow,
  LinkedRecordTableTitle,
  type LinkedRecordTableBodyProps,
  type LinkedRecordTableCellProps,
  type LinkedRecordTableOpenCellProps,
  type LinkedRecordTableProps,
  type LinkedRecordTableRowProps,
  type LinkedRecordTableTitleProps,
} from "./components/custom/linked-record-table";
export {
  ChatMessageBubble,
  type ChatMessageBubbleProps,
  type ChatMessageRole,
  type ChatMessageAlign,
} from "./components/custom/chat-message-bubble";
export {
  ToolCallRow,
  type ToolCallRowProps,
  type ToolCallRowSectionProps,
  type ToolCallStatus,
} from "./components/custom/tool-call-row";
export {
  ToolCallStatusIcon,
  type ToolCallStatusIconProps,
} from "./components/custom/tool-call-status-icon";
export { Metric, type MetricProps, type MetricTone } from "./components/custom/metric";
export {
  MetricGrid,
  type MetricGridColumns,
  type MetricGridProps,
} from "./components/custom/metric-grid";
export {
  Avatar,
  AvatarBadge,
  AvatarFallback,
  AvatarGroup,
  AvatarGroupCount,
  AvatarImage,
  type AvatarShape,
  type AvatarSize,
} from "./components/avatar";
export { ButtonGroup, ButtonGroupSeparator, ButtonGroupText } from "./components/button-group";
export { buttonGroupVariants } from "./components/button-group-variants";
export {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldHeader,
  FieldLabel,
  FieldLegend,
  FieldSeparator,
  FieldSet,
  FieldTitle,
} from "./components/field";
export {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupText,
  InputGroupTextarea,
} from "./components/input-group";
export {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemFooter,
  ItemGroup,
  ItemHeader,
  ItemMedia,
  ItemSeparator,
  ItemSelectionIndicator,
  ItemTitle,
  type ItemAs,
  type ItemIndicator,
  type ItemIndicatorTone,
  type ItemProps,
  type ItemSelectionIndicatorProps,
} from "./components/item";
export { NativeSelect, NativeSelectOptGroup, NativeSelectOption } from "./components/native-select";
export { Checkbox } from "./components/checkbox";
export { RadioGroup, RadioGroupItem } from "./components/radio-group";
export { Surface, type SurfaceProps, type SurfaceSize } from "./components/surface";
export {
  useStepper,
  useStepItem,
  Stepper,
  StepperItem,
  StepperTrigger,
  StepperRail,
  StepperBody,
  StepperIndicator,
  StepperSeparator,
  StepperTitle,
  StepperDescription,
  StepperPanel,
  StepperContent,
  StepperNav,
  type StepperProps,
  type StepperItemProps,
  type StepperTriggerProps,
  type StepperContentProps,
} from "./components/reui/stepper";
export { Tree, TreeItem, TreeItemLabel, TreeDragLine } from "./components/reui/tree";
export type {
  TreeProps,
  TreeItemProps,
  TreeItemLabelProps,
  TreeDragLineProps,
} from "./components/reui/tree";
export { Filters, FiltersWithSearch } from "./components/reui/filters";
export { createFilter } from "./components/reui/hooks/filter-helpers";
export type { Filter, FilterFieldsConfig, FilterFieldConfig } from "./components/reui/filters";
export { Textarea, type TextareaProps, type TextareaVariant } from "./components/textarea";
export { Toaster, type ToasterProps } from "./components/sonner";
export { toast } from "./components/sonner-toast";
export { DirectionProvider, useDirection } from "./components/direction";
export {
  GithubStars,
  GithubStarsIcon,
  GithubStarsLogo,
  GithubStarsNumber,
  GithubStarsParticles,
  useGithubStars,
  type GithubStarsContextType,
  type GithubStarsIconProps,
  type GithubStarsLogoProps,
  type GithubStarsNumberProps,
  type GithubStarsParticlesProps,
  type GithubStarsProps,
} from "./components/animation/github-stars";
export {
  Particles,
  ParticlesEffect,
  useParticles,
  type ParticlesEffectProps,
  type ParticlesProps,
} from "./components/animation/particles";
export { SlidingNumber, type SlidingNumberProps } from "./components/animation/sliding-number";
