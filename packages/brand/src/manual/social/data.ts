// Chapter 07, Social: networks, profile assets, caption rules and example posts.

export const networks = [
  {
    name: "GitHub",
    handle: "github.com/compozy",
    priority: "Highest",
    role: "Where the product lives",
    job: "The repo is the landing page for developers: README, releases, discussions. Every other network points here.",
    cadence: "Every release; README reviewed monthly",
  },
  {
    name: "X",
    handle: "@compozy",
    priority: "High",
    role: "Where builders talk",
    job: "Releases, short mechanism threads, replies to people building with agents. The founder account carries the voice; the brand account carries releases.",
    cadence: "3–5 posts a week, replies daily",
  },
  {
    name: "Discord",
    handle: "discord.gg/compozy",
    priority: "High",
    role: "Where users get help",
    job: "Support, release notes, office hours, the AI-engineering servers where Compozy shows up as a member, not an advertiser.",
    cadence: "Every release; office hours weekly",
  },
  {
    name: "YouTube",
    handle: "@compozy",
    priority: "Medium",
    role: "Where it is shown working",
    job: "Screen-recorded walkthroughs, one task per video, and talks. The archive developers send each other.",
    cadence: "2 videos a month",
  },
  {
    name: "LinkedIn",
    handle: "linkedin.com/company/compozy",
    priority: "Lower",
    role: "Where managers read",
    job: "The process argument for engineering managers: repeatable agent work across a team. Founder posts lead; the company page mirrors them.",
    cadence: "2 posts a week",
  },
];

export const assets: [network: string, asset: string, size: string, safe: string, id: string][] = [
  ["X", "Avatar", "400×400", "circle", "x-avatar"],
  ["X", "Header", "1500×500", "middle 60% of height; avatar covers lower left", "x-header"],
  ["X", "Post image", "1600×900", "16:9, 100 px margin", "x-post"],
  ["X", "Quote card", "1600×900", "16:9, 100 px margin", "x-quote"],
  [
    "LinkedIn",
    "Personal banner",
    "1584×396",
    "right 68%; photo covers lower left",
    "linkedin-banner",
  ],
  ["LinkedIn", "Company cover", "1128×191", "right 74%; logo covers lower left", "linkedin-cover"],
  ["LinkedIn", "Company logo", "400×400", "rounded square", "linkedin-avatar"],
  ["LinkedIn", "Post image", "1200×1200", "90 px margin", "linkedin-post"],
  ["LinkedIn · X", "Carousel slide", "1080×1350", "90 px margin", "carousel-cover"],
  ["YouTube", "Channel art", "2560×1440", "1546×423 centred", "youtube-banner"],
  ["YouTube", "Channel picture", "800×800", "circle", "youtube-avatar"],
  ["YouTube", "Thumbnail", "1280×720", "bottom-right timestamp", "youtube-thumb"],
  ["YouTube · X", "Vertical cover", "1080×1920", "top 250 px, bottom 340 px", "story"],
  ["GitHub", "Social preview", "1280×640", "40 px border", "github-social-preview"],
  ["GitHub", "Org avatar", "500×500", "rounded square", "github-avatar"],
  ["GitHub", "README header", "1280×320", "light and dark themes", "readme-header"],
  ["Discord", "Server banner", "960×540", "shown at ~240×135", "discord-banner"],
  ["Discord", "Server icon", "512×512", "circle", "discord-icon"],
  ["All", "Release card", "1600×900", "16:9, 100 px margin", "release-card"],
];

export const avatarRules = [
  "Symbol on ink, never the lockup: the wordmark is unreadable at 32 px.",
  "Symbol width 56% of the square, centred; it stays inside the inscribed circle (70.7%) on every crop.",
  "Same master on every network so the avatar is recognised in a mention list at 24 px.",
  "No frames, no gradient ring, no seasonal versions. Change it only with a manual release.",
];
export const bannerRules = [
  "One line from the messaging bank, never a feature list. The bio does the listing.",
  "Text clears the avatar zone and the mobile crop; the visual may run under both.",
  "An Atmosphere visual (Swarm, Converge, Terrain, Horizon, Cone or Block) fills the side away from the text, with an ink gradient between them. Product pieces (README, GitHub preview, thumbnail, release card) show the Live desktop or agents list instead.",
  "Update the line when the positioning changes, not for every release.",
];

export const xRules: [string, string][] = [
  [
    "Account",
    "Brand account posts releases and docs; the founder account posts opinions and the build. The brand quotes the founder, never the other way round.",
  ],
  [
    "Format",
    "One idea per post. Image only when it adds a fact: a release card, a product crop, a quote card.",
  ],
  [
    "Threads",
    "Up to 6 posts. Post 1 stands alone; the last post carries the link. Numbering “1/” is fine, the thread emoji is not.",
  ],
  [
    "Replies",
    "Answer questions in the reply with a command or a doc link. Never reply to rivals’ launches with Compozy.",
  ],
  [
    "Links",
    "Link in the first post for releases (the card unfurls); in the last post for threads.",
  ],
];

export const linkedinRules: [string, string][] = [
  [
    "Who posts",
    "The founder posts; the company page reshares with one line of context. Personal posts outperform pages and read as human.",
  ],
  [
    "First 210 characters",
    "Everything before “see more” must work alone: the problem and the claim, no warm-up.",
  ],
  [
    "Line breaks",
    "Short paragraphs of 1–3 sentences. No one-word lines, no arrow lists, no bold Unicode.",
  ],
  ["Images", "One square image or a carousel. Never a stock photo of a meeting."],
  ["Tags", "Tag people only when they are in the post and agreed. Never tag to borrow reach."],
];

export const carouselMap: [n: string, name: string, text: string, id: string][] = [
  [
    "01",
    "Cover",
    "The pain in one line: five terminals and the prompt in the wrong window.",
    "carousel-cover",
  ],
  ["02", "Control plane", "One desktop for every agent, with the desktop visual.", "carousel-body"],
  ["03", "Always-on", "Agents that check in on their own, with the faces.", "carousel-body"],
  ["04", "Handoff", "A session continues in another harness, with the picker.", "carousel-body"],
  ["05", "Approvals", "The agent asks, you decide, with the permission dock.", "carousel-body"],
  [
    "06",
    "Action",
    "One action and where to read more; the install line is a small detail.",
    "carousel-cta",
  ],
];

export const thumbRules = [
  "Four words or fewer, Instrument Sans 400, at least 7.5% of the width tall.",
  "The Compozy desktop, flat, cropped to what the video is about: the faces, a session or the board. Never a terminal window.",
  "Readable at 168×94 px: check it at that size before uploading.",
  "Bottom-right 14% stays clear of text: the timestamp sits there.",
  "Never a face pulling an expression, never arrows or red circles.",
];

export const githubRules: [string, string][] = [
  [
    "Description",
    "The positioning line, under 120 characters, no emoji: “The autonomous control plane for your agents. Local-first, MIT.”",
  ],
  [
    "Topics",
    "agents, ai-agents, agent-orchestration, developer-tools, golang, local-first, claude-code, codex.",
  ],
  [
    "Social preview",
    "Upload github-social-preview in Settings → General. It shows the desktop and is what Slack, X and Discord show when the repo is linked.",
  ],
  [
    "Releases",
    "Title “v0.6.0”, body starts with one sentence of outcome, then grouped changes. The release card goes at the top of the body.",
  ],
  [
    "Org avatar",
    "github-avatar at 500×500. GitHub rounds organisation avatars to a square with 6 px radius.",
  ],
];

export const readmeSnippet = `<p align="center">
  <img src="./.github/assets/readme-header.png"
       alt="Compozy: the autonomous control plane for your agents."
       width="100%" />
</p>`;

export const anatomy = [
  {
    part: "Hook",
    words: "≤ 12 words",
    text: "Compozy 0.6: agents now check in on their own.",
    note: "The outcome, in the reader’s terms. No question, no tease.",
  },
  {
    part: "Mechanism",
    words: "1–2 sentences",
    text: "Always-on agents work between triggers and call you only when they need a decision. The board shows who has the next step.",
    note: "How it works, with the product as the subject.",
  },
  {
    part: "Proof",
    words: "1 artifact",
    text: "A crop of the desktop: the always-on agents list",
    note: "A product crop, a Live visual or a number with a date. A command only when the post is about the CLI.",
  },
  {
    part: "Link",
    words: "1 URL",
    text: "compozy.com/changelog",
    note: "One link, at the end, where the reader goes next.",
  },
];

export const formulas: [string, string][] = [
  ["Release", "{Version} {outcome}. {Mechanism in one sentence}. {Link}."],
  ["Mechanism", "{Problem the reader has}. {Capability}. {Command or crop}."],
  [
    "Field note",
    "{What the founder saw, with a number and a date}. {What it means}. {What Compozy does about it}.",
  ],
  ["Honest status", "{What ships today}. {What does not, yet}. {Where to follow}."],
];

export const lengths: [string, string][] = [
  ["X", "≤ 280 characters per post; release posts ≤ 200 so the card has room."],
  ["LinkedIn", "600–1,300 characters; the first 210 carry the post."],
  ["YouTube title", "≤ 60 characters; the outcome first, “Compozy” last or not at all."],
  [
    "YouTube description",
    "First 2 lines (≤ 150 characters) answer “what will I learn”; chapters after.",
  ],
  ["GitHub release", "One sentence of outcome, then grouped changes. No marketing paragraph."],
  ["Discord", "≤ 6 lines in #announcements; link to the release, not a copy of it."],
];

export const posts = [
  {
    net: "X · brand",
    kind: "Release",
    text: "Compozy 0.6 is out.\n\nAlways-on agents check in on their own and call you only when they need a decision. The board shows who has the next step.\n\nNotes and install: github.com/compozy/compozy/releases",
  },
  {
    net: "X · founder",
    kind: "Mechanism",
    text: "Every session on the board sits in the column of who has the next step: you, the agent, a reviewer, or nobody.\n\nThat one rule replaces the five terminals and the prompt in the wrong window.\n\ncompozy.com",
  },
  {
    net: "LinkedIn · founder",
    kind: "Field note",
    text: "I have taught 3,000 developers to work with agents. The bottleneck stopped being the model. It became managing the sessions: five terminals, and the prompt in the wrong window.\n\nCompozy is the control plane for that. One desktop where you see every agent and who has the next step. Always-on agents check in on their own and call you only when needed. A session can hand its work to another harness, and anything outside an agent’s permissions waits for your approval.\n\nIt is open source and in beta: github.com/compozy/compozy",
  },
  {
    net: "GitHub · release",
    kind: "Release notes",
    text: "v0.6.0\n\nAlways-on agents check in on their own and call you only when needed, and a session can continue in another harness.\n\nAdded\n- Always-on agents, set per agent profile\n- Continue with another agent, with context, files and worktree\n\nChanged\n- Child sessions run in their own worktrees by default",
  },
  {
    net: "YouTube · description",
    kind: "Walkthrough",
    text: "Run Claude Code, Codex and Cursor from one desktop, set up an always-on agent and approve its first request.\n\n00:00 Five terminals\n01:10 One desktop for every agent\n04:30 An agent that checks in on its own\n08:05 Approving right in the session\n\nDocs: compozy.com/docs",
  },
  {
    net: "Discord · #announcements",
    kind: "Release",
    text: "Compozy 0.6 is out: always-on agents and handoff across harnesses.\n\nUpgrade: brew upgrade compozy\nNotes: github.com/compozy/compozy/releases\n\nOffice hours on Thursday at 17:00 UTC in #voice if anything breaks.",
  },
];

export const forbidden: [string, string][] = [
  [
    "Hashtags",
    "None, on every network. They read as marketing and add nothing to discovery for this audience.",
  ],
  ["Emoji", "None, including the thread emoji, rocket, sparkles and checkmarks used as bullets."],
  ["Exclamation marks", "None. Calm is the brand."],
  ["“We’re excited to announce”", "Start with the thing. Nobody needs the feeling."],
  [
    "Hype words",
    "Revolutionary, seamless, game-changing, next-gen, AI-powered, effortless, cutting-edge.",
  ],
  [
    "Engagement bait",
    "“Agree?”, “Thoughts?”, “Comment YES for the link”, polls with no real question.",
  ],
  ["Customers", "Design partners are never customers, users or logos. No implied revenue."],
  [
    "Rivals",
    "Never name or compare to another product in a post. The enemy is the DIY agent stack.",
  ],
  ["Undated numbers", "Stars, users and members always carry a month: “2.7k stars, Nov 2026”."],
  [
    "Terminal as the image",
    "The terminal is the before picture (“five terminals”). Product posts show the desktop; one install line under a link is fine.",
  ],
  [
    "Retired words",
    "“Stays on” (use Always-on), “wrapper”, “wedge”, “AI factory”, “24/7”, “simple” without a metric.",
  ],
];

/** Field values of the four carousel body slides (passed to the carousel-body kit template). */
export const carouselBodies: Record<"step" | "title" | "body" | "visual" | "count", string>[] = [
  {
    step: "02",
    title: "One desktop for every agent.",
    body: "Claude Code, Codex, Gemini CLI, OpenCode and Cursor in one control plane. You see all of them, and command all of them, from the same place.",
    visual: "desktop",
    count: "02 / 06",
  },
  {
    step: "03",
    title: "Agents that check in on their own.",
    body: "Always-on agents work between triggers and call you only when they need a decision. The rest of the time, the faces rest.",
    visual: "agents",
    count: "03 / 06",
  },
  {
    step: "04",
    title: "Continue with another agent.",
    body: "A session moves to another harness with its context, files and worktree. Start in Claude Code, finish in Codex.",
    // No handoff surface exists in web/ yet (ADR-005): the orchestrator capture shows the hand-off.
    visual: "orchestrate",
    count: "04 / 06",
  },
  {
    step: "05",
    title: "The agent asks. You decide.",
    body: "Anything outside an agent’s permissions waits in the session: allow once, always allow in this project, or deny.",
    visual: "approval",
    count: "05 / 06",
  },
];
