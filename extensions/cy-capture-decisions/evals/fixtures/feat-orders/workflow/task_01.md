---
status: completed
title: Implement the order event store
type: feature
---

# Implement the order event store

Append-only event stream for orders with a projected read model. Verified: `/cy-final-verify` passed
(p99 < 200ms on the projection replay benchmark).
