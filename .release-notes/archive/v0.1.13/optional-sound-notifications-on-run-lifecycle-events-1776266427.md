---
title: Optional sound notifications on run lifecycle events
type: feature
---

Opt-in audio cues that play when a run completes or fails, so you can step away from long-running sessions without missing the result. Ships **disabled by default** — no sound unless you explicitly enable it.

### Setup

Add a `[sound]` section to `.compozy/config.toml` (project or global):

```toml
[sound]
enabled = true
on_completed = "glass"   # plays on successful completion
on_failed = "basso"      # plays on failure or cancellation
```

### Built-in presets

Seven presets work cross-platform out of the box:

| Preset      | macOS                                   | Linux                                        | Windows                             |
| ----------- | --------------------------------------- | -------------------------------------------- | ----------------------------------- |
| `glass`     | `/System/Library/Sounds/Glass.aiff`     | `freedesktop/stereo/complete.oga`            | `Media\Windows Notify Calendar.wav` |
| `basso`     | `/System/Library/Sounds/Basso.aiff`     | `freedesktop/stereo/dialog-error.oga`        | `Media\chord.wav`                   |
| `ping`      | `/System/Library/Sounds/Ping.aiff`      | `freedesktop/stereo/message.oga`             | `Media\notify.wav`                  |
| `hero`      | `/System/Library/Sounds/Hero.aiff`      | `freedesktop/stereo/bell.oga`                | `Media\tada.wav`                    |
| `funk`      | `/System/Library/Sounds/Funk.aiff`      | `freedesktop/stereo/bell.oga`                | `Media\Ring06.wav`                  |
| `tink`      | `/System/Library/Sounds/Tink.aiff`      | `freedesktop/stereo/message.oga`             | `Media\ding.wav`                    |
| `submarine` | `/System/Library/Sounds/Submarine.aiff` | `freedesktop/stereo/phone-incoming-call.oga` | `Media\ringin.wav`                  |

### Custom sounds

Pass an absolute path to use your own audio file:

```toml
[sound]
enabled = true
on_completed = "/Users/you/sounds/success.wav"
on_failed = "/Users/you/sounds/fail.wav"
```

### Lifecycle events

| Event         | Config field   | When it fires                                  |
| ------------- | -------------- | ---------------------------------------------- |
| Run completed | `on_completed` | Task finishes successfully                     |
| Run failed    | `on_failed`    | Task errors out                                |
| Run cancelled | `on_failed`    | Task is interrupted (reuses the failure sound) |

Playback is synchronous with a 3-second timeout — a missing or slow audio file never blocks shutdown. Errors are logged at debug level and never surface to the user.
