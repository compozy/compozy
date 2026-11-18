---
title: Choose the interface font, text size and letter spacing
type: feat
---

**Settings → Appearance → Text** lets you tune how the interface reads:

- **Interface font**: Figtree (default), Geist, Inter or Instrument Sans. A bundled font downloads when you select it or open the font list to preview it.
- **Text size**: Small, Default, Large or Larger. Text and line height scale together. Terminals keep their own size.
- **Letter spacing**: Tight, Default, Relaxed or Wide.

Like the font choice, these settings apply to every web and desktop client connected to the daemon, and the app starts with them already applied. You can also set `text_size` and `letter_spacing` under `[shell.appearance]` in `config.toml`.

Migration notes: no action needed; existing installs keep the default font, size and spacing.
