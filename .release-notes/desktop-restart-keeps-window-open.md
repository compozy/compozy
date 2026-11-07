---
title: Restarting CompozyOS from Settings keeps the app open
type: fix
---

Some settings ask you to restart CompozyOS. Clicking **Restart** used to close the whole desktop app whenever the restart took more than a few seconds, for example with busy sessions or a data migration, and you had to open it again by hand. The window now stays open. It shows that CompozyOS is restarting, reconnects to the new daemon and returns you to where you were. Runtime updates that restart the daemon get the same treatment.
