---
title: The desktop boot window names a config file error
type: fix
---

When CompozyOS cannot load your `config.toml`, the desktop boot window now says so. It used to report that the daemon was "listening but unhealthy", retry five times, and end on a generic start failure, even when the running daemon was healthy. It now stops at once, names the file and the settings it does not recognize or the line of invalid TOML, and adds **Open config file** next to **Try again** and **Open logs**.

Migration notes: no action needed.
