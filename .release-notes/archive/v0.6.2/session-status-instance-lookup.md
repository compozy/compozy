---
title: Session status uses less CPU and memory
type: fix
---

CompozyOS now reads only the instance binding it needs when checking a session's status, reducing repeated database work and memory allocations in busy daemons with large session catalogs. Session status and access rules stay the same.
