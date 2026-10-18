# CH-settings-skills-automation-sections: Save independent runtime settings without losing another section

```yaml
charter:
  id: CH-settings-skills-automation-sections
  mission: "As Dora, edit Skills and Automation through their public settings pages and HTTP/UDS APIs, reject invalid values, then reload and independently verify each saved section while unrelated settings remain intact."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-administer-runtime-settings
  scenarios: [MS-037]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Read both sections and one unrelated section before making changes. Save a valid Skills value from Web and read it over HTTP and UDS."
      - "Save a valid Automation value through HTTP or UDS and verify the public Web control after a reload. Exercise the other transport as a writer too."
      - "Submit a wrong-type value and an invalid bounded value. Require a clear refusal and preservation of the last valid settings."
      - "Restore the initial settings through public interfaces and independently read both sections again."
    must_avoid:
      - "Private database or implementation inspection during the walk, editing runtime files to bypass validation, starting automation work, or changing provider credentials."
```

The 2026-10-02 matrix originally associated MS-037 with a clarification-timeout charter that
does not list this revised scenario. This charter owns the current Skills/Automation acceptance
contract without changing the original scenario inventory.
