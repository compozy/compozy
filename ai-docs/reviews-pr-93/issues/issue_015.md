# Issue 015

- Status: RESOLVED
- Disposition: VALID
- File: `internal/setup/extensions.go`
- Review request: make the relationship between `Skill.SourceFS` and `extensionSkillSource.Source` explicit.
- Validation: both fields were initialized from the same `fs.FS` value independently.
- Action taken: now construct the `Skill` first and set `extensionSkillSource.Source` from `extensionSkill.SourceFS`.
