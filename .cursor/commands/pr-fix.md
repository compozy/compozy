<arguments>
  <type>string</type>
  <pr>integer</pr>
  <from>string</from>
</arguments>
<arguments_table>
| Argument | Type     | Description                                 |
|----------|----------|---------------------------------------------|
| --type   | string   | Type of item: issue, duplicated, outside, nitpick |
| --pr     | integer  | Pull request number                         |
| --from   | string   | Range of comments to address                |
</arguments_table>

<critical>
- **YOU NEED** to fix the $ARGUMENTS:type from $ARGUMENTS:from in the ai-docs/reviews-pr-$ARGUMENTS:pr, and only finish when ALL THESE ISSUES are addressed;
- This should be fixed in THE BEST WAY possible, not using workarounds;
- **YOU MUST** follow project standards and rules from .cursor/rules, and ensure all items in <arguments> are addressed;
- If, in the end, you don't have all issues from <arguments> fixed, your work will be **INVALIDATED**;
- After making all the changes, you need to update the progress in the _summary.md file and all the related $ARGUMENTS:type.md files.
- **MUST DO:** If $ARGUMENTf:type is `issue`, after resolving every issue run `scripts/resolve_pr_issues.sh --pr-dir ai-docs/reviews-pr-$ARGUMENTS:pr --from <start> --to <end>` so the script calls `gh` to close the review threads and refreshes the summary.
</critical>
