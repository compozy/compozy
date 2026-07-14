# Architecture Depth Map (active)

# @import'd into agent memory. Route behavior INTO deep modules; do NOT widen seams;

# do NOT re-propose avoided deepenings. Detail: .compozy/arch-reviews/<area>.md

## apps/web | audited 2026-07-13 | report .compozy/arch-reviews/apps-web.md

seam | apps/web/router | Do not widen the router integration seam.
deep | apps/web/navigation | Route new navigation behavior through this module.
