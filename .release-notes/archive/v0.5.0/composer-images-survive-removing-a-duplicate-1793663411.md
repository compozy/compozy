---
title: Removing a composer image no longer breaks another one
type: fix
---

Removing an image from the composer could delete the stored file shared with another image, including a duplicate tile or one already sent, so Send then failed with `attachment not found`. Removing a tile now only removes the tile. Retention and session deletion clean up the stored file, so an abandoned upload can remain until retention runs.

A prompt refused for a missing attachment now reads as plain guidance instead of a raw error. Your text and files return to the composer, the file uploads again, and Send retries. The guidance stays with the draft until the message is accepted or you remove the attachment or discard the draft.

A full page reload still keeps only your draft text, not unsent files, so attach them again.
