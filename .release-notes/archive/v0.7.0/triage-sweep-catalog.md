---
title: Lower background session maintenance overhead
type: fix
---

CompozyOS now reads only triage sessions during periodic triage cleanup, reducing background CPU and metadata-cache pressure for large session histories. Deleted session metadata is evicted without waiting for a full session listing, including when a metadata read overlaps deletion. Archive and retention behavior stays unchanged.
