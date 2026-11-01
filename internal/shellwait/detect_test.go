package shellwait

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	denied := map[string]struct {
		command string
		kind    Kind
	}{
		"gh pr checks watch":             {`gh pr checks 123 --watch`, KindWatch},
		"gh pr checks watch with flags":  {`gh pr checks 123 --watch --fail-fast --interval 30`, KindWatch},
		"gh pr checks with repo flag":    {`gh pr checks 123 -R compozy/compozy --watch`, KindWatch},
		"gh watch under timeout":         {`timeout 570 gh pr checks 123 --watch --interval 30`, KindWatch},
		"gh watch under timeout options": {`gtimeout -k 5 -s TERM 570 gh pr checks 123 --watch`, KindWatch},
		"gh run watch":                   {`gh run watch 1234 --exit-status`, KindWatch},
		"gh run watch absolute path":     {`/opt/homebrew/bin/gh run watch 1234`, KindWatch},
		"watch after a pipe":             {`gh run watch 1 | tail -n 5`, KindWatch},
		"watch after and":                {`cd web && gh pr checks 9 --watch`, KindWatch},
		"watch in a subshell":            {`(gh run watch 1)`, KindWatch},
		"watch in a substitution":        {`out=$(gh pr checks 9 --watch)`, KindWatch},
		"watch in a bash string":         {`bash -lc 'gh pr checks 9 --watch'`, KindWatch},
		"quoted watch flag":              {`gh pr checks 9 "--watch"`, KindWatch},
		"watch binary":                   {`watch -n 5 kubectl get pods`, KindWatch},
		"tail follow":                    {`tail -f app.log`, KindWatch},
		"tail follow cluster":            {`tail -n 20 -F app.log`, KindWatch},
		"sleep at the threshold":         {`sleep 5`, KindSleep},
		"long sleep":                     {`sleep 300`, KindSleep},
		"absolute sleep":                 {`/bin/sleep 6`, KindSleep},
		"sleep with suffix":              {`sleep 2m`, KindSleep},
		"fractional sleep":               {`sleep 5.5`, KindSleep},
		"summed sleep":                   {`sleep 3 3`, KindSleep},
		"sleep forever":                  {`sleep infinity`, KindSleep},
		"sleep before a check":           {`sleep 30 && gh run view 1`, KindSleep},
		"backgrounded sleep":             {`sleep 30 &`, KindSleep},
		"env sleep":                      {`env FOO=1 sleep 30`, KindSleep},
		"assignment sleep":               {`FOO=1 sleep 30`, KindSleep},
		"nohup sleep":                    {`nohup sleep 30`, KindSleep},
		"command sleep":                  {`command sleep 30`, KindSleep},
		"nested shell sleep":             {`sh -c "bash -c 'sleep 30'"`, KindSleep},
		"eval sleep":                     {`eval "sleep 30"`, KindSleep},
		"seq polling loop":               {`for i in $(seq 1 27); do s=$(gh run view 1); /bin/sleep 2; done`, KindPoll},
		"brace polling loop":             {`for i in {1..10}; do curl -f x && break; sleep 1; done`, KindPoll},
		"c style polling loop":           {`for ((i=0;i<30;i++)); do check; sleep 2; done`, KindPoll},
		"until loop":                     {`until curl -sf localhost:3000; do sleep 1; done`, KindPoll},
		"while true loop":                {`while true; do gh run view 1; sleep 0.5; done`, KindPoll},
		"while with absolute sleep":      {`while pgrep node; do /bin/sleep 1; done`, KindPoll},
		"loop with variable delay":       {`while true; do check; sleep "$DELAY"; done`, KindPoll},
		"multiline loop":                 {"until test -f done; do\n  sleep 2\ndone", KindPoll},
		"for loop with unknown items":    {`for i in $(jobs); do check; sleep 1; done`, KindPoll},
		"for loop with unknown delay":    {`for i in a b; do sleep $n; done`, KindPoll},
		"loop inside a shell string":     {`sh -c 'until test -f x; do sleep 1; done'`, KindPoll},
		"single long pause in a loop":    {`for i in 1; do sleep 5; done`, KindPoll},
	}
	for name, tc := range denied {
		t.Run("Should deny "+name, func(t *testing.T) {
			t.Parallel()
			finding, found := Detect(tc.command)
			if !found {
				t.Fatalf("Detect(%q) allowed, want %s", tc.command, tc.kind)
			}
			if finding.Kind != tc.kind {
				t.Fatalf("Detect(%q) kind = %s, want %s", tc.command, finding.Kind, tc.kind)
			}
		})
	}

	allowed := map[string]string{
		"short sleep":                  `sleep 1`,
		"short sleep in a test":        `sleep 1 && curl localhost:3000/health`,
		"sub threshold sleep":          `sleep 4.9`,
		"variable sleep alone":         `sleep "$DELAY"`,
		"git log":                      `git log --oneline -5`,
		"gh pr checks":                 `gh pr checks 123`,
		"gh pr checks json":            `gh pr checks 123 --json name,state`,
		"gh pr checks watch false":     `gh pr checks 123 --watch=false`,
		"gh help":                      `gh pr checks --watch --help`,
		"gh run view":                  `gh run view 123 --log-failed`,
		"gh run list":                  `gh run list --limit 5`,
		"gh pr view watch word":        `gh pr view watch`,
		"quoted mention":               `echo "gh pr checks 123 --watch"`,
		"commit message mention":       `git commit -m "use gh run watch; sleep 30"`,
		"single quoted mention":        `echo 'while true; do sleep 1; done'`,
		"search pattern":               `rg -n 'sleep 30' internal`,
		"grep flag mention":            `grep -- --watch README.md`,
		"heredoc mention":              "cat <<'EOF'\nsleep 300\ngh run watch 1\nEOF",
		"file named sleep":             `cat sleep`,
		"bun watch script":             `bun run watch`,
		"tsc watch flag":               `tsc --watch`,
		"project watch script":         `./watch build`,
		"project sleep script":         `./sleep 30`,
		"which sleep":                  `command -v sleep`,
		"tail lines":                   `tail -n 20 app.log`,
		"tail default":                 `tail app.log`,
		"plain loop":                   `for f in *.go; do gofmt -l "$f"; done`,
		"read loop with delay":         `while read -r line; do echo "$line"; sleep 1; done < file`,
		"brief pause across few items": `for i in 1 2; do run "$i"; sleep 1; done`,
		"loop tiny sleep":              `for f in a b c; do touch "$f"; sleep 0.1; done`,
		"while without sleep":          `while test -f lock; do rm lock; done`,
		"go test timeout":              `timeout 120 go test ./internal/shellwait`,
		"empty":                        ``,
		"unparsable":                   `if then fi (((`,
		"timeout with no command":      `timeout 5`,
		"shell without command string": `bash script.sh`,
		"shell with a dynamic string":  `sh -c "$CMD"`,
		"unknown env wrapper":          `env`,
		"go run with sleep flag":       `go run ./cmd/x --sleep 300`,
	}
	for name, command := range allowed {
		t.Run("Should allow "+name, func(t *testing.T) {
			t.Parallel()
			if finding, found := Detect(command); found {
				t.Fatalf("Detect(%q) = %+v, want allowed", command, finding)
			}
		})
	}
}

func TestDetectPowerShell(t *testing.T) {
	t.Parallel()

	denied := map[string]struct {
		command string
		kind    Kind
	}{
		"start sleep":      {`Start-Sleep -Seconds 60`, KindSleep},
		"start sleep bare": {`Start-Sleep 30`, KindSleep},
		"sleep alias":      {`sleep 30`, KindSleep},
		"gh watch":         {`gh pr checks 123 --watch`, KindWatch},
		"gh exe watch":     {`gh.exe run watch 123`, KindWatch},
		"polling while":    {`while ($true) { gh run view 1; Start-Sleep -Seconds 2 }`, KindPoll},
		"polling do":       {`do { Test-Path x; Start-Sleep 1 } while ($true)`, KindPoll},
	}
	for name, tc := range denied {
		t.Run("Should deny "+name, func(t *testing.T) {
			t.Parallel()
			finding, found := DetectPowerShell(tc.command)
			if !found || finding.Kind != tc.kind {
				t.Fatalf("DetectPowerShell(%q) = %+v, %v, want %s", tc.command, finding, found, tc.kind)
			}
		})
	}

	allowed := map[string]string{
		"short sleep":     `Start-Sleep -Seconds 1`,
		"millis sleep":    `Start-Sleep -Milliseconds 500`,
		"gh checks":       `gh pr checks 123`,
		"quoted mention":  `Write-Output "gh pr checks 1 --watch; Start-Sleep 60"`,
		"plain foreach":   `foreach ($f in $files) { Write-Output $f }`,
		"variable sleep":  `Start-Sleep -Seconds $delay`,
		"quick loop wait": `foreach ($f in $files) { Start-Sleep -Milliseconds 100 }`,
	}
	for name, command := range allowed {
		t.Run("Should allow "+name, func(t *testing.T) {
			t.Parallel()
			if finding, found := DetectPowerShell(command); found {
				t.Fatalf("DetectPowerShell(%q) = %+v, want allowed", command, finding)
			}
		})
	}
}

func TestDenyReason(t *testing.T) {
	t.Parallel()

	t.Run("Should name the exact unwrapped argv for a watch", func(t *testing.T) {
		t.Parallel()
		finding, _ := Detect(`timeout 570 gh pr checks 123 --watch --interval 30`)
		reason := DenyReason(finding)
		for _, want := range []string{
			`compozy__terminal_exec {"args":["pr","checks","123","--watch","--interval","30"],"command":"gh"}`,
			`compozy__monitor_create {"kind":"terminal"`,
			"still_running",
		} {
			if !strings.Contains(reason, want) {
				t.Fatalf("reason = %q, want %q", reason, want)
			}
		}
	})

	t.Run("Should move a polling loop whole into the terminal", func(t *testing.T) {
		t.Parallel()
		command := `until test -f "done"; do sleep 1; done`
		finding, _ := Detect(command)
		encoded, err := json.Marshal(command)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		want := `{"args":["-c",` + string(encoded) + `],"command":"sh"}`
		if reason := DenyReason(finding); !strings.Contains(reason, want) {
			t.Fatalf("reason = %q, want %q", reason, want)
		}
	})

	t.Run("Should offer a timer within monitor bounds for a delay", func(t *testing.T) {
		t.Parallel()
		for command, want := range map[string]string{
			`sleep 6`:        `"delay_seconds":6`,
			`sleep 1.5m`:     `"delay_seconds":90`,
			`sleep infinity`: `"delay_seconds":86400`,
		} {
			finding, _ := Detect(command)
			if reason := DenyReason(finding); !strings.Contains(reason, want) {
				t.Fatalf("DenyReason(%q) = %q, want %q", command, reason, want)
			}
		}
	})
}

func TestDetectToolInput(t *testing.T) {
	t.Parallel()

	denied := map[string]string{
		"Bash string":     `{"command":"gh pr checks 1 --watch"}`,
		"Codex argv":      `{"command":["bash","-lc","gh run watch 1"],"cwd":"/tmp"}`,
		"terminal create": `{"command":"gh","args":["pr","checks","1","--watch"]}`,
		"terminal in sh":  `{"command":"sh","args":["-c","sleep 30"]}`,
	}
	for name, input := range denied {
		t.Run("Should deny "+name, func(t *testing.T) {
			t.Parallel()
			if _, found := DetectToolInput(json.RawMessage(input)); !found {
				t.Fatalf("DetectToolInput(%s) allowed, want denied", input)
			}
		})
	}

	allowed := map[string]string{
		"short command":   `{"command":"git status"}`,
		"terminal args":   `{"command":"echo","args":["sleep 30"]}`,
		"no command":      `{"path":"/tmp/x"}`,
		"malformed input": `{"command":`,
		"numeric command": `{"command":12}`,
	}
	for name, input := range allowed {
		t.Run("Should allow "+name, func(t *testing.T) {
			t.Parallel()
			if finding, found := DetectToolInput(json.RawMessage(input)); found {
				t.Fatalf("DetectToolInput(%s) = %+v, want allowed", input, finding)
			}
		})
	}
}

func TestRunHook(t *testing.T) {
	t.Parallel()

	hookOutput := func(t *testing.T, input string) string {
		t.Helper()
		var stdout bytes.Buffer
		RunHook(strings.NewReader(input), &stdout)
		return stdout.String()
	}

	t.Run("Should deny a watch with a valid PreToolUse decision", func(t *testing.T) {
		t.Parallel()
		output := hookOutput(t, `{"tool_name":"Bash","tool_input":{"command":"gh pr checks 1 --watch \"x\""}}`)
		var decision struct {
			HookSpecificOutput struct {
				HookEventName            string `json:"hookEventName"`
				PermissionDecision       string `json:"permissionDecision"`
				PermissionDecisionReason string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(output), &decision); err != nil {
			t.Fatalf("hook output %q decode error = %v", output, err)
		}
		got := decision.HookSpecificOutput
		if got.HookEventName != "PreToolUse" || got.PermissionDecision != "deny" ||
			!strings.Contains(got.PermissionDecisionReason, "compozy__terminal_exec") {
			t.Fatalf("decision = %+v, want a deny naming compozy__terminal_exec", got)
		}
	})

	t.Run("Should use PowerShell rules for the PowerShell tool", func(t *testing.T) {
		t.Parallel()
		input := `{"tool_name":"PowerShell","tool_input":{"command":"while ($true) { Start-Sleep 2 }"}}`
		if output := hookOutput(t, input); !strings.Contains(output, `\"powershell\"`) {
			t.Fatalf("output = %q, want a PowerShell redirect", output)
		}
	})

	t.Run("Should stay silent so the command runs", func(t *testing.T) {
		t.Parallel()
		for _, input := range []string{
			`{"tool_name":"Bash","tool_input":{"command":"git log"}}`,
			`{"tool_name":"Bash","tool_input":{}}`,
			`not json`,
			``,
		} {
			if output := hookOutput(t, input); output != "" {
				t.Fatalf("hook input %q output = %q, want silence", input, output)
			}
		}
	})
}
