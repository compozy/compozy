---
title: Swap two windows of a split from the Window menu, the palette, or a shortcut
type: feature
---

Two windows of a split could trade places only by dragging one header onto the other with Shift held.

- **Window › Swap.** With a window in a split focused, **Swap ▸** lists **Left**, **Right**, **Up** and **Down** for each side that has a neighbour. The focused window trades places with that neighbour and keeps focus.
- **From the command palette.** Type `swap` and choose **Swap left**, **Swap right**, **Swap up** or **Swap down**. A side with no window does nothing; swaps never wrap around.
- **Your own shortcut.** The four commands (`window.swap.left`, `.right`, `.up`, `.down`) ship unbound. Assign a chord in Settings › Shortcuts. Existing shortcut settings load unchanged.
- **One undo step.** Layout undo puts both windows back.
