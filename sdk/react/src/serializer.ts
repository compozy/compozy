import type {
  Chip,
  DetailBody,
  FormBody,
  FormField,
  GridBody,
  GridSection,
  GridTile,
  Image,
  Row,
  Section,
  ViewChrome,
  ViewBadge,
  ViewPayload,
} from "@compozy/extension-sdk";

import type { HandlerRegistry } from "./handler-registry.js";
import { childNodes, firstChildNode, isHostNode } from "./host-types.js";
import type { HostChild, HostNode } from "./host-types.js";
import { serializeActions } from "./serialize-actions.js";

export interface SerializedView {
  payload: ViewPayload;
  handlers: string[];
}

export function serializeView(children: HostChild[], registry: HandlerRegistry): SerializedView {
  const root = children.filter(isHostNode).find(node => !node.hidden);
  if (!root) throw new Error("a view program must render one root component");
  registry.beginFrame();
  const payload = serializeRoot(root, registry);
  const handlers = registry.activeIDs();
  registry.endFrame();
  return { payload, handlers };
}

function serializeRoot(root: HostNode, handlers: HandlerRegistry): ViewPayload {
  switch (root.type) {
    case "view-list":
      return serializeList(root, handlers);
    case "view-detail": {
      const payload: ViewPayload = { view: "v1", detail: serializeDetail(root, handlers) };
      if (root.props.isLoading === true) payload.chrome = { is_loading: true };
      return payload;
    }
    case "view-grid":
      return serializeGrid(root, handlers);
    case "view-form":
      return { view: "v1", form: serializeForm(root, handlers) };
    default:
      throw new Error(`unsupported view root: ${root.type}`);
  }
}

function serializeList(root: HostNode, handlers: HandlerRegistry): ViewPayload {
  const sections = childNodes(root, "view-list-section").map(section => {
    const value: Section = {
      rows: childNodes(section, "view-list-item").map(row => serializeRow(row, handlers)),
    };
    const title = optionalString(section.props.title);
    if (title) value.title = title;
    return value;
  });
  const directRows = childNodes(root, "view-list-item");
  if (directRows.length > 0) {
    sections.unshift({ rows: directRows.map(row => serializeRow(row, handlers)) });
  }
  const payload: ViewPayload = {
    view: "v1",
    chrome: serializeChrome(root, handlers),
    sections,
  };
  if (Array.isArray(root.props.chips)) payload.chips = root.props.chips as Chip[];
  const empty = firstChildNode(root, "view-list-empty");
  if (empty) {
    payload.empty = { title: requiredString(empty.props.title, "empty view title") };
    const hint = optionalString(empty.props.hint);
    const icon = optionalString(empty.props.icon);
    if (hint) payload.empty.hint = hint;
    if (icon) payload.empty.icon = icon;
  }
  return payload;
}

function serializeRow(node: HostNode, handlers: HandlerRegistry): Row {
  const result: Row = {
    id: requiredString(node.props.id, "list item id"),
    title: requiredString(node.props.title, "list item title"),
  };
  const subtitle = optionalString(node.props.subtitle);
  const icon = optionalString(node.props.icon);
  if (subtitle) result.subtitle = subtitle;
  if (icon) result.icon = icon;
  if (isRecord(node.props.badge)) result.badge = parseBadge(node.props.badge);
  if (Array.isArray(node.props.keywords)) result.keywords = node.props.keywords as string[];
  if (Array.isArray(node.props.accessories)) {
    result.accessories = node.props.accessories as string[];
  }
  const detail = firstChildNode(node, "view-list-item-detail");
  if (detail) result.detail = serializeDetail(detail, handlers);
  const actions = serializeActions(node, handlers);
  if (actions.length > 0) result.actions = actions;
  return result;
}

function serializeDetail(node: HostNode, handlers: HandlerRegistry): DetailBody {
  const result: DetailBody = {};
  const markdown = optionalString(node.props.markdown);
  if (node.props.isLoading === true) result.is_loading = true;
  if (markdown) result.markdown = markdown;
  if (Array.isArray(node.props.metadata)) {
    result.metadata = node.props.metadata as NonNullable<DetailBody["metadata"]>;
  }
  const actions = serializeActions(node, handlers);
  if (actions.length > 0) result.actions = actions;
  return result;
}

function serializeGrid(root: HostNode, handlers: HandlerRegistry): ViewPayload {
  const sections = childNodes(root, "view-grid-section").map(section => {
    const value: GridSection = {
      tiles: childNodes(section, "view-grid-item").map(item => serializeGridItem(item, handlers)),
    };
    const title = optionalString(section.props.title);
    if (title) value.title = title;
    return value;
  });
  const direct = childNodes(root, "view-grid-item");
  if (direct.length > 0) {
    sections.unshift({ tiles: direct.map(item => serializeGridItem(item, handlers)) });
  }
  const grid: GridBody = { sections };
  return { view: "v1", chrome: serializeChrome(root, handlers), grid };
}

function serializeGridItem(node: HostNode, handlers: HandlerRegistry): GridTile {
  if (!isRecord(node.props.image)) throw new Error("grid item image is required");
  const result: GridTile = {
    id: requiredString(node.props.id, "grid item id"),
    title: requiredString(node.props.title, "grid item title"),
    image: parseImage(node.props.image),
  };
  if (isRecord(node.props.badge)) result.badge = parseBadge(node.props.badge);
  const actions = serializeActions(node, handlers);
  if (actions.length > 0) result.actions = actions;
  return result;
}

function serializeForm(root: HostNode, handlers: HandlerRegistry): FormBody {
  const result: FormBody = {
    fields: childNodes(root, "view-form-field").map(field => serializeFormField(field, handlers)),
  };
  const submit = serializeActions(root, handlers).find(action => action.submit_form === true);
  const onSubmit = handlers.bind(root, "onSubmit", root.props.onSubmit);
  if (submit) result.submit = submit;
  if (onSubmit) result.on_submit = onSubmit;
  return result;
}

function serializeFormField(node: HostNode, handlers: HandlerRegistry): FormField {
  const result: FormField = {
    id: requiredString(node.props.id, "form field id"),
    type: requiredString(node.props.type, "form field type"),
    label: requiredString(node.props.label, "form field label"),
  };
  const placeholder = optionalString(node.props.placeholder);
  const error = optionalString(node.props.error);
  if (placeholder) result.placeholder = placeholder;
  if (node.props.required === true) result.required = true;
  if (Array.isArray(node.props.options)) result.options = node.props.options as string[];
  else {
    const options = childNodes(node, "view-form-option").map(option =>
      requiredString(option.props.value, "dropdown option value")
    );
    if (options.length > 0) result.options = options;
  }
  if (node.props.directories === true) result.directories = true;
  if (node.props.defaultValue !== undefined) result.default = node.props.defaultValue as never;
  if (error) result.error = error;
  const onChange = handlers.bind(node, "onChange", node.props.onChange);
  const onBlur = handlers.bind(node, "onBlur", node.props.onBlur);
  if (onChange) result.on_change = onChange;
  if (onBlur) result.on_blur = onBlur;
  return result;
}

function serializeChrome(node: HostNode, handlers: HandlerRegistry): ViewChrome {
  const result: ViewChrome = {};
  const searchText = optionalString(node.props.searchText);
  const placeholder = optionalString(node.props.searchBarPlaceholder);
  const activeChip = optionalString(node.props.activeChip);
  if (node.props.isLoading === true) result.is_loading = true;
  if (searchText) result.search_text = searchText;
  if (placeholder) result.search_placeholder = placeholder;
  if (typeof node.props.throttle === "number") result.throttle_ms = node.props.throttle;
  else if (node.props.throttle === true) result.throttle_ms = 150;
  const onSearch = handlers.bind(node, "onSearchTextChange", node.props.onSearchTextChange);
  if (typeof node.props.filtering === "boolean") result.filtering = node.props.filtering;
  else if (onSearch) result.filtering = false;
  if (node.props.complete === true) result.complete = true;
  if (activeChip) result.active_chip = activeChip;
  if (typeof node.props.columns === "number") result.columns = node.props.columns;
  if (isRecord(node.props.pagination)) {
    result.pagination = { has_more: node.props.pagination.hasMore === true };
    if (typeof node.props.pagination.pageSize === "number") {
      result.pagination.page_size = node.props.pagination.pageSize;
    }
    const onLoadMore = handlers.bind(node, "onLoadMore", node.props.pagination.onLoadMore);
    if (onLoadMore) result.on_load_more = onLoadMore;
  }
  const onChip = handlers.bind(node, "onChipToggle", node.props.onChipToggle);
  const onSelection = handlers.bind(node, "onSelectionChange", node.props.onSelectionChange);
  if (onSearch) result.on_search = onSearch;
  if (onChip) result.on_chip = onChip;
  if (onSelection) result.on_selection = onSelection;
  return result;
}

function parseBadge(record: Record<string, unknown>): ViewBadge {
  return {
    label: requiredString(record.label, "badge label"),
    tone: requiredString(record.tone, "badge tone"),
  };
}

function parseImage(record: Record<string, unknown>): Image {
  const url = optionalString(record.url);
  const token = optionalString(record.token);
  const emoji = optionalString(record.emoji);
  if (url) return { url };
  if (token) return { token };
  if (emoji) return { emoji };
  throw new Error("grid item image requires url, token, or emoji");
}

function requiredString(value: unknown, field: string): string {
  const result = optionalString(value);
  if (!result) throw new Error(`${field} is required`);
  return result;
}

function optionalString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
