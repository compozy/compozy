# CH-beta-install-channels: Install the same pinned beta through every published channel

```yaml
charter:
  id: CH-beta-install-channels
  mission: "As Dora, install the documented v0.3 beta through the hosted installer, npm beta tag, and pinned Go module in isolated destinations, proving each yields the same version and no public surface advertises Homebrew."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-evaluate-compozy-beta
  scenarios: [REL-beta-install-paths]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Install each channel into a disposable prefix without replacing the operator binary; compare compozy version output and the explicit bootstrap boundary."
      - "Read README and the site installation surfaces from the user entry points and search them for a Homebrew path."
    must_avoid:
      - "Publishing, tagging, mutating registries, or replacing the operator's installed Compozy binary."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
