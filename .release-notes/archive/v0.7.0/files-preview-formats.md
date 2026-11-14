---
title: Preview Markdown, tables, HTML, notebooks, PDF and media in Files
type: feat
---

The Files viewer now previews the files agents write and read instead of showing raw source or a "binary" state. Markdown, SVG, CSV/TSV, HTML, Mermaid and Jupyter notebooks get a `Source | Preview | Split` toggle in the header; Preview is the default and the chosen mode is remembered per file kind. `Cmd/Ctrl+Shift+V` flips the mode, and `Cmd/Ctrl+F` in Preview switches to Source with Find open.

PDFs, video, audio and large images now open in place. Relative Markdown links open their target in the viewer and relative images load from the same folder. HTML previews always run sandboxed without scripts.

Migration notes: no action needed.
