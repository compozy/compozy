package cli

type completionArgSpec struct {
	kind completionKind
	file bool
}

type completionCommandSpec struct {
	args     []completionArgSpec
	variadic bool
}

func singleArgSpec(kind completionKind) completionCommandSpec {
	return completionCommandSpec{args: []completionArgSpec{{kind: kind}}}
}

func variadicArgSpec(kind completionKind) completionCommandSpec {
	return completionCommandSpec{args: []completionArgSpec{{kind: kind}}, variadic: true}
}

var completionCommandTable = map[string]completionCommandSpec{
	// Session commands
	"session stop":               singleArgSpec(kindSession),
	"session remove":             singleArgSpec(kindSession),
	"session status":             singleArgSpec(kindSession),
	"session resume":             singleArgSpec(kindSession),
	"session recap":              singleArgSpec(kindSession),
	"session repair":             singleArgSpec(kindSession),
	"session approve":            singleArgSpec(kindSession),
	"session archive":            singleArgSpec(kindSession),
	"session unarchive":          singleArgSpec(kindSession),
	"session later":              singleArgSpec(kindSession),
	"session resume-later":       singleArgSpec(kindSession),
	"session pin":                singleArgSpec(kindSession),
	"session unpin":              singleArgSpec(kindSession),
	"session mark-seen":          singleArgSpec(kindSession),
	"session mcp":                singleArgSpec(kindSession),
	"session mcp check":          singleArgSpec(kindSession),
	"session commands":           singleArgSpec(kindSession),
	"session compact":            singleArgSpec(kindSession),
	"session continue":           singleArgSpec(kindSession),
	"session fork":               singleArgSpec(kindSession),
	"session changes":            singleArgSpec(kindSession),
	"session inspect":            singleArgSpec(kindSession),
	"session health":             singleArgSpec(kindSession),
	"session soul refresh":       singleArgSpec(kindSession),
	"session clarify pending":    singleArgSpec(kindSession),
	"session clarify answer":     singleArgSpec(kindSession),
	"session goal set":           singleArgSpec(kindSession),
	"session goal replace":       singleArgSpec(kindSession),
	"session goal status":        singleArgSpec(kindSession),
	"session goal pause":         singleArgSpec(kindSession),
	"session goal stop":          singleArgSpec(kindSession),
	"session goal resume":        singleArgSpec(kindSession),
	"session goal clear":         singleArgSpec(kindSession),
	"session input list":         singleArgSpec(kindSession),
	"session input edit":         singleArgSpec(kindSession),
	"session input steer":        singleArgSpec(kindSession),
	"session input cancel":       singleArgSpec(kindSession),
	"session input clear":        singleArgSpec(kindSession),
	"session interactions":       singleArgSpec(kindSession),
	"session pending":            singleArgSpec(kindSession),
	"session link":               singleArgSpec(kindSession),
	"session unlink":             singleArgSpec(kindSession),
	"session move":               singleArgSpec(kindSession),
	"session reorder":            variadicArgSpec(kindSession),
	"session move-to-profile":    {args: []completionArgSpec{{kind: kindSession}, {kind: kindProfile}}},
	"session search":             singleArgSpec(kindSession),
	"session outline":            singleArgSpec(kindSession),
	"session pr":                 singleArgSpec(kindSession),
	"session processes":          singleArgSpec(kindSession),
	"session processes stop":     singleArgSpec(kindSession),
	"session prompt-cancel":      singleArgSpec(kindSession),
	"session prompt":             singleArgSpec(kindSession),
	"session events":             singleArgSpec(kindSession),
	"session history":            singleArgSpec(kindSession),
	"session rename":             singleArgSpec(kindSession),
	"session reparent":           singleArgSpec(kindSession),
	"session rewind":             singleArgSpec(kindSession),
	"session runtime set":        singleArgSpec(kindSession),
	"session runtime clear":      singleArgSpec(kindSession),
	"session side open":          singleArgSpec(kindSession),
	"session side list":          singleArgSpec(kindSession),
	"session subagents":          singleArgSpec(kindSession),
	"session usage":              singleArgSpec(kindSession),
	"session wait":               singleArgSpec(kindSession),
	"session worktree bind":      {args: []completionArgSpec{{kind: kindSession}, {kind: kindWorktree}}},
	"session worktree unbind":    singleArgSpec(kindSession),
	"session attachments upload": {args: []completionArgSpec{{kind: kindSession}, {file: true}}},
	"session folder rename":      singleArgSpec(kindSessionFolder),
	"session folder delete":      singleArgSpec(kindSessionFolder),
	"session folder reorder":     variadicArgSpec(kindSessionFolder),

	// Workspace commands
	"workspace info":           singleArgSpec(kindWorkspace),
	"workspace edit":           singleArgSpec(kindWorkspace),
	"workspace remove":         singleArgSpec(kindWorkspace),
	"workspace archive":        singleArgSpec(kindWorkspace),
	"workspace unarchive":      singleArgSpec(kindWorkspace),
	"workspace move":           singleArgSpec(kindWorkspace),
	"workspace reorder":        variadicArgSpec(kindWorkspace),
	"workspace folder rename":  singleArgSpec(kindWorkspaceFolder),
	"workspace folder delete":  singleArgSpec(kindWorkspaceFolder),
	"workspace folder reorder": variadicArgSpec(kindWorkspaceFolder),

	// Agent commands
	"agent info":                        singleArgSpec(kindAgent),
	"agent activity":                    singleArgSpec(kindAgent),
	"agent update":                      singleArgSpec(kindAgent),
	"agent delete":                      singleArgSpec(kindAgent),
	"agent duplicate":                   singleArgSpec(kindAgent),
	"agent soul inspect":                singleArgSpec(kindAgent),
	"agent soul validate":               singleArgSpec(kindAgent),
	"agent soul write":                  singleArgSpec(kindAgent),
	"agent soul delete":                 singleArgSpec(kindAgent),
	"agent soul history":                singleArgSpec(kindAgent),
	"agent soul rollback":               singleArgSpec(kindAgent),
	"agent heartbeat inspect":           singleArgSpec(kindAgent),
	"agent heartbeat validate":          singleArgSpec(kindAgent),
	"agent heartbeat write":             singleArgSpec(kindAgent),
	"agent heartbeat delete":            singleArgSpec(kindAgent),
	"agent heartbeat history":           singleArgSpec(kindAgent),
	"agent heartbeat rollback":          singleArgSpec(kindAgent),
	"agent heartbeat status":            singleArgSpec(kindAgent),
	"agent heartbeat wake":              singleArgSpec(kindAgent),
	"agent heartbeat authority show":    singleArgSpec(kindAgent),
	"agent heartbeat authority approve": singleArgSpec(kindAgent),
	"agent heartbeat authority revoke":  singleArgSpec(kindAgent),
	"agent heartbeat sources":           singleArgSpec(kindAgent),
	"agent heartbeat sources approve":   singleArgSpec(kindAgent),
	"agent heartbeat sources revoke":    singleArgSpec(kindAgent),
	"agent memory inspect":              singleArgSpec(kindAgent),
	"agent memory add":                  singleArgSpec(kindAgent),
	"agent memory replace":              singleArgSpec(kindAgent),
	"agent memory remove":               singleArgSpec(kindAgent),
	"agent memory write":                singleArgSpec(kindAgent),
	"agent memory validate":             singleArgSpec(kindAgent),
	"agent memory search":               singleArgSpec(kindAgent),
	"agent memory history":              singleArgSpec(kindAgent),
	"agent memory rollback":             singleArgSpec(kindAgent),
	"agent instance create":             singleArgSpec(kindAgent),
	"agent instance show":               singleArgSpec(kindAgent),
	"agent instance pause":              singleArgSpec(kindAgent),
	"agent instance resume":             singleArgSpec(kindAgent),
	"agent instance archive":            singleArgSpec(kindAgent),
	"agent instance adopt":              singleArgSpec(kindAgent),
	"agent instance ensure-main":        singleArgSpec(kindAgent),
	"agent instance clear-main":         singleArgSpec(kindAgent),
	"agent instance reports":            singleArgSpec(kindAgent),
	"agent instance check":              singleArgSpec(kindAgent),
	"agent instance away":               singleArgSpec(kindAgent),
	"agent instance mark-seen":          singleArgSpec(kindAgent),
	"agent instance check-ins":          singleArgSpec(kindAgent),
	"agent instance budget":             singleArgSpec(kindAgent),
	"agent instance checks on":          singleArgSpec(kindAgent),
	"agent instance checks off":         singleArgSpec(kindAgent),
	"agent instance pick":               singleArgSpec(kindAgent),

	// Provider and provider models commands
	"provider inspect":        singleArgSpec(kindProvider),
	"provider auth status":    singleArgSpec(kindProvider),
	"provider auth login":     singleArgSpec(kindProvider),
	"provider models list":    singleArgSpec(kindProvider),
	"provider models set":     {args: []completionArgSpec{{kind: kindProvider}, {kind: kindModel}}},
	"provider models refresh": singleArgSpec(kindProvider),
	"provider models status":  singleArgSpec(kindProvider),
	"provider usage list":     singleArgSpec(kindProvider),
	"provider usage refresh":  singleArgSpec(kindProvider),

	// Profile commands
	"profile use":       singleArgSpec(kindProfile),
	"profile update":    singleArgSpec(kindProfile),
	"profile rename":    singleArgSpec(kindProfile),
	"profile archive":   singleArgSpec(kindProfile),
	"profile unarchive": singleArgSpec(kindProfile),
	"profile delete":    singleArgSpec(kindProfile),

	// Task commands (task runs and reviews are opted out below)
	"task get":                  singleArgSpec(kindTask),
	"task inspect":              singleArgSpec(kindTask),
	"task update":               singleArgSpec(kindTask),
	"task delete":               singleArgSpec(kindTask),
	"task publish":              singleArgSpec(kindTask),
	"task start":                singleArgSpec(kindTask),
	"task approve":              singleArgSpec(kindTask),
	"task reject":               singleArgSpec(kindTask),
	"task cancel":               singleArgSpec(kindTask),
	"task block":                singleArgSpec(kindTask),
	"task unblock":              singleArgSpec(kindTask),
	"task blocks":               singleArgSpec(kindTask),
	"task recover":              singleArgSpec(kindTask),
	"task pause":                singleArgSpec(kindTask),
	"task resume":               singleArgSpec(kindTask),
	"task fan-out":              singleArgSpec(kindTask),
	"task child create":         singleArgSpec(kindTask),
	"task dependency add":       singleArgSpec(kindTask),
	"task dependency remove":    {args: []completionArgSpec{{kind: kindTask}, {kind: kindTask}}},
	"task profile inspect":      singleArgSpec(kindTask),
	"task profile update":       singleArgSpec(kindTask),
	"task profile set-worktree": singleArgSpec(kindTask),
	"task profile delete":       singleArgSpec(kindTask),
	"task run list":             singleArgSpec(kindTask),
	"task run enqueue":          singleArgSpec(kindTask),

	// Automation job commands
	"automation jobs get":     singleArgSpec(kindAutomation),
	"automation jobs update":  singleArgSpec(kindAutomation),
	"automation jobs delete":  singleArgSpec(kindAutomation),
	"automation jobs trigger": singleArgSpec(kindAutomation),
	"automation jobs history": singleArgSpec(kindAutomation),

	// Worktree commands
	"worktree cancel":      singleArgSpec(kindWorktree),
	"worktree inspect":     singleArgSpec(kindWorktree),
	"worktree status":      singleArgSpec(kindWorktree),
	"worktree remove":      variadicArgSpec(kindWorktree),
	"worktree dismiss":     singleArgSpec(kindWorktree),
	"worktree deliver":     singleArgSpec(kindWorktree),
	"worktree exit":        singleArgSpec(kindWorktree),
	"worktree commit":      singleArgSpec(kindWorktree),
	"worktree push":        singleArgSpec(kindWorktree),
	"worktree pr":          singleArgSpec(kindWorktree),
	"worktree exit-cancel": singleArgSpec(kindWorktree),

	// Extension commands
	"extension remove":        singleArgSpec(kindExtension),
	"extension update":        singleArgSpec(kindExtension),
	"extension enable":        singleArgSpec(kindExtension),
	"extension disable":       singleArgSpec(kindExtension),
	"extension status":        singleArgSpec(kindExtension),
	"extension provenance":    singleArgSpec(kindExtension),
	"extension inventory":     singleArgSpec(kindExtension),
	"extension commands":      singleArgSpec(kindExtension),
	"extension reload":        {args: []completionArgSpec{{kind: kindExtension}, {file: true}}},
	"extension logs":          singleArgSpec(kindExtension),
	"extension exec":          singleArgSpec(kindExtension),
	"extension secrets set":   singleArgSpec(kindExtension),
	"extension secrets bind":  singleArgSpec(kindExtension),
	"extension secrets list":  singleArgSpec(kindExtension),
	"extension secrets unset": singleArgSpec(kindExtension),
	"extension setup":         singleArgSpec(kindExtension),

	// Computer commands
	"computer status":  singleArgSpec(kindComputer),
	"computer enable":  singleArgSpec(kindComputer),
	"computer disable": singleArgSpec(kindComputer),
	"computer remove":  singleArgSpec(kindComputer),
	"computer routes":  singleArgSpec(kindComputer),
	"computer update":  singleArgSpec(kindComputer),

	// Vault commands
	"vault get":    singleArgSpec(kindVault),
	"vault put":    singleArgSpec(kindVault),
	"vault delete": singleArgSpec(kindVault),

	// Toolset commands
	"toolsets info": singleArgSpec(kindToolset),
}

var completionCommandOptOut = map[string]struct{}{
	// New values / user-authored strings
	"agent create":            {}, // new agent name
	"agent draft":             {}, // natural-language goal
	"app-token create":        {}, // new app token name
	"extension init":          {}, // new extension name
	"notify":                  {}, // notification title
	"profile create":          {}, // new profile name
	"session folder create":   {}, // new folder name
	"skill create":            {}, // new skill name
	"workspace folder create": {}, // new folder name
	"worktree create":         {}, // new worktree name

	// File/directory arguments (keep Cobra default file completion)
	"app open":           {}, // file or directory path
	"extension build":    {}, // directory path
	"extension dev":      {}, // directory path
	"extension install":  {}, // local path or marketplace source
	"extension publish":  {}, // generation directory path
	"extension validate": {}, // directory path
	"workspace add":      {}, // workspace directory path
	"worktree adopt":     {}, // linked worktree path

	// Commands with built-in ValidArgs
	"daemon profile": {}, // defines static ValidArgs

	// Operation IDs, un-listable run IDs, and other non-entity IDs
	"layout-profile delete":                {}, // window layout profile ID, not an agent profile
	"layout-profile get":                   {}, // window layout profile ID, not an agent profile
	"layout-profile put":                   {}, // window layout profile ID, not an agent profile
	"profile ops retry":                    {}, // profile operation ID
	"account token revoke":                 {}, // account token ID
	"agent instance occurrence":            {}, // occurrence ID (not globally listable)
	"agent skill install-conflict resolve": {}, // install conflict operation ID
	"agent skill review show":              {}, // skill review ID
	"agent skill review approve":           {}, // skill review ID
	"agent skill review reject":            {}, // skill review ID
	"app-token update":                     {}, // app token ID
	"app-token revoke":                     {}, // app token ID
	"approvals show":                       {}, // tool approval request ID
	"approvals cancel":                     {}, // tool approval request ID
	"approvals resolve":                    {}, // tool approval request ID
	"automation runs get":                  {}, // automation run ID (not listable)
	"automation runs cancel":               {}, // automation run ID (not listable)
	"automation suggestions accept":        {}, // suggestion ID
	"automation suggestions dismiss":       {}, // suggestion ID
	"automation triggers get":              {}, // trigger ID
	"automation triggers update":           {}, // trigger ID
	"automation triggers delete":           {}, // trigger ID
	"automation triggers history":          {}, // trigger ID
	"cmd-palette alias set":                {}, // palette command ID
	"cmd-palette alias clear":              {}, // palette command ID
	"cmd-palette bind":                     {}, // palette command ID
	"cmd-palette unbind":                   {}, // palette command ID
	"cmd-palette inspect":                  {}, // palette command ID
	"cmd-palette invoke":                   {}, // palette command ID
	"cmd-palette pin":                      {}, // palette command ID
	"cmd-palette unpin":                    {}, // palette command ID
	"computer add account":                 {}, // account computer ID not yet added locally
	"computer add ssh":                     {}, // SSH host
	"computer setup answer":                {}, // setup operation ID
	"computer setup status":                {}, // setup operation ID
	"computer setup cancel":                {}, // setup operation ID
	"config get":                           {}, // config key path
	"config set":                           {}, // config key path
	"config unset":                         {}, // config key path
	"extension restore":                    {}, // permanently removed built-in name
	"extension search":                     {}, // search query
	"hooks info":                           {}, // hook name
	"loop approve":                         {}, // loop run ID (run-scoped, not listable)
	"loop events":                          {}, // loop run ID (run-scoped, not listable)
	"loop why":                             {}, // loop run ID (run-scoped, not listable)
	"marketplace categories":               {}, // category query
	"marketplace info":                     {}, // marketplace entry ID
	"marketplace search":                   {}, // search query
	"marketplace sources add":              {}, // source ref
	"marketplace sources refresh":          {}, // source name
	"marketplace sources remove":           {}, // source name
	"mcp allow":                            {}, // MCP server name
	"mcp auth login":                       {}, // MCP server name
	"mcp auth status":                      {}, // MCP server name
	"mcp auth logout":                      {}, // MCP server name
	"mcp check":                            {}, // MCP server name
	"mcp disable":                          {}, // MCP server name
	"mcp enable":                           {}, // MCP server name
	"mcp import":                           {}, // runtime/server ref
	"remote devices approve":               {}, // remote device ID
	"remote devices deny":                  {}, // remote device ID
	"remote devices revoke":                {}, // remote device ID
	"remote devices scopes":                {}, // remote device ID
	"resource delete":                      {}, // resource kind + ID
	"resource get":                         {}, // resource kind + ID
	"resource put":                         {}, // resource kind + ID
	"roles show":                           {}, // role name
	"secret rm":                            {}, // providers/<p>/<slot> or extensions/<e>/<k>, not vault: ref
	"secret set":                           {}, // providers/<p>/<slot> or extensions/<e>/<k>, not vault: ref
	"session subagents show":               {}, // session-scoped subagent ID
	"session subagents cancel":             {}, // session-scoped subagent ID
	"session subagents delete":             {}, // session-scoped subagent ID
	"skill disable":                        {}, // skill name
	"skill enable":                         {}, // skill name
	"skill expose":                         {}, // skill name
	"skill info":                           {}, // skill name
	"skill unexpose":                       {}, // skill name
	"skill view":                           {}, // skill name
	"skill where":                          {}, // skill name
	"task complete":                        {}, // task run ID (not globally listable)
	"task fail":                            {}, // task run ID (not globally listable)
	"task heartbeat":                       {}, // task run ID (not globally listable)
	"task release":                         {}, // task run ID (not globally listable)
	"task retry":                           {}, // task run ID (not globally listable)
	"task review request":                  {}, // task run ID (not globally listable)
	"task review show":                     {}, // task review ID
	"task review submit":                   {}, // task review ID
	"task run attach-session":              {}, // task run ID (not globally listable)
	"task run cancel":                      {}, // task run ID (not globally listable)
	"task run complete":                    {}, // task run ID (not globally listable)
	"task run fail":                        {}, // task run ID (not globally listable)
	"task run recover":                     {}, // task run ID (not globally listable)
	"task run result":                      {}, // task run ID (not globally listable)
	"task run show":                        {}, // task run ID (not globally listable)
	"task run start":                       {}, // task run ID (not globally listable)
	"terminal attach":                      {}, // terminal ID
	"terminal exec":                        {}, // executable command and args
	"terminal get":                         {}, // terminal ID
	"terminal kill":                        {}, // terminal ID
	"terminal quote":                       {}, // terminal ID
	"terminal record start":                {}, // terminal ID
	"terminal record stop":                 {}, // terminal ID
	"terminal respond":                     {}, // terminal ID
	"terminal signal":                      {}, // terminal ID
	"tool approve":                         {}, // tool ID
	"tool approvals revoke":                {}, // grant ID
	"tool approvals set":                   {}, // tool ID
	"tool artifact read":                   {}, // artifact URI
	"tool info":                            {}, // tool ID
	"tool invoke":                          {}, // tool ID
	"tool search":                          {}, // search query
	"window activate":                      {}, // window ID
	"window pin":                           {}, // window ID
	"window reorder":                       {}, // window ID
	"window unpin":                         {}, // window ID
	"__internal extension-provider":        {}, // internal hidden command
}
