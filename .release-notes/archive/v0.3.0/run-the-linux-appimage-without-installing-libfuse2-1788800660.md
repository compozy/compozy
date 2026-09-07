---
title: Run the Linux AppImage without installing libfuse2
type: fix
---

Linux AppImages now embed the static runtime, allowing distributions with FUSE 3 to launch the app without installing `libfuse2`. Release, local, and update-test packaging share the same runtime selection. The extraction fallback remains available for environments without kernel FUSE support.

PR: [#548](https://github.com/compozy/compozy/pull/548).
