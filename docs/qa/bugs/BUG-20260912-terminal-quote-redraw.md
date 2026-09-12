# Terminal quotes include raw cursor redraws

- **Impact:** Blocks-Completion.
- **Scenario:** ET-terminal-shell-config-fidelity, step 9.
- **Status:** fixed; native retest passed; browser CI retest tracked by the integration report.

CI run `34676447555`, Web shard 4 (`103507101483`), rendered both ASCII and Unicode prompts
correctly but quoted `a\babc` instead of visible `abc`. The line-range read selected raw ring bytes
without applying cursor movement. Removing ANSI sequences in the assertion cannot interpret a
backspace and would not repair the produced excerpt.

PTY line reads now render only retained bytes through the existing VT emulator, then apply the
requested range, grep and byte cap. Raw tail/pipe data, authorization and public schemas are unchanged.
The existing VT suite checks redraw, erase, wide characters and scrolled rows; the terminal read
suite checks range/grep routing, raw-byte preservation and buffer trimming.

The isolated real-zsh walk used the public CLI attachment and quote commands plus HTTP screen
readback. Separate `a`, `b`, `c` inputs yielded `❯ abc` and `&gt; abc` in escaped quotes. Backspace
yielded `❯ ab`; a subsequent `printf` completed. Evidence: `quote-native-unicode.json`,
`quote-native-backspace.json`, `quote-native-ascii.json`, `quote-native-screen.json` and
`quote-native-command.json` under the September 12 integration lab.
