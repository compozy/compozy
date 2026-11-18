// Project files: Pierre-backed tree, single-file viewer, and multi-file diff.
// Pierre (`@pierre/trees`, `@pierre/diffs`) is imported only inside this
// package (ADR-001); the highlight worker constructor lives at
// `@compozy/ui/lib/code-highlight-worker` so this barrel stays bundler-neutral.
export {
  FileTree,
  type FileTreeDecoration,
  type FileTreeDecorations,
  type FileTreeProps,
  type FileTreeRowMenuControls,
  type FileTreeRowMenuItem,
  type FileTreeStatus,
} from "../components/files/file-tree";
export {
  CodeViewer,
  type CodeViewerMatch,
  type CodeViewerProps,
} from "../components/files/code-viewer";
export {
  DiffViewer,
  type DiffViewerChangeType,
  type DiffViewerContents,
  type DiffViewerFileHeaderState,
  type DiffViewerHandle,
  type DiffViewerItem,
  type DiffViewerItemStatus,
  type DiffViewerProps,
} from "../components/files/diff-viewer";
export {
  FileImagePreview,
  type FileImagePreviewProps,
} from "../components/files/file-image-preview";
export {
  CodeHighlightPoolProvider,
  type CodeHighlightPoolProviderProps,
} from "../components/files/code-highlight-pool";
export {
  FileTablePreview,
  type FileTablePreviewProps,
} from "../components/files/file-table-preview";
export {
  delimiterForPath,
  parseDelimited,
  type DelimitedTable,
} from "../components/files/delimited-text";
export { FileHtmlPreview, type FileHtmlPreviewProps } from "../components/files/file-html-preview";
export { prepareHtmlPreview } from "../components/files/html-preview-document";
export {
  FileMediaPreview,
  type FileMediaPreviewProps,
} from "../components/files/file-media-preview";
export { FilePdfPreview, type FilePdfPreviewProps } from "../components/files/file-pdf-preview";
export {
  FileNotebookPreview,
  type FileNotebookPreviewProps,
} from "../components/files/file-notebook-preview";
