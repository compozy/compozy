# Compozy: The Ultimate Agentic Orchestration Platform

## Ditch Fragile AI Code. Let Compozy Manage the Chaos.

Launch your first AI agent workflow in minutes using intuitive YAML—powered by Compozy's **Go-based engine** for lightning-fast execution and seamless parallelism via goroutines.

```yaml
# Process thousands of documents in parallel with just 20 lines
resource: workflow
id: document-analysis
version: "1.0.0"

tasks:
  - id: process_documents
    type: parallel
    with:
      documents: "{{ .workflow.input.documents }}"
    tasks:
      - id: extract_entities
        $use: agent(local::agents.#(id=="extractor"))
        action: extract
        with:
          document: "{{ .item }}"

      - id: classify_content
        $use: agent(local::agents.#(id=="classifier"))
        action: classify
        with:
          text: "{{ .item.content }}"

      - id: generate_summary
        $use: agent(local::agents.#(id=="summarizer"))
        action: summarize
        with:
          content: "{{ .item.content }}"

outputs:
  results: "{{ .tasks.process_documents.outputs }}"
  total_processed: "{{ len(.workflow.input.documents) }}"
```

<p align="center">
  <a href="#get-started" class="cta-primary">Deploy Your First Workflow Now →</a>
</p>

<p align="center" class="trust-line">
  ⭐ Rapidly Growing on GitHub | 🛡️ Trusted by Innovative Teams | 📈 Millions of Executions Processed
</p>

---

## Discover Compozy

**Compozy is an open-source platform that transforms declarative YAML into scalable, distributed AI agent applications.** Leveraging Temporal's robust infrastructure and a **Go backend**, it simplifies LLM integrations, error recovery, and state persistence while harnessing goroutine concurrency for unmatched performance.

---

## The Real Risks of Building AI Orchestration In-House

Scaling AI isn't just about models—it's about resilient systems. DIY approaches often lead to hidden pitfalls that derail projects:

### 🔥 **Over 40% of agentic AI projects will be canceled by 2027** due to high costs and unclear value

_Source: Gartner, 2025_
<argument name="citation_id">18</argument>

### 💸 **$50,000 to $500,000+ average investment** in custom AI solutions, with enterprises reaching $2M+

_Including data prep, infrastructure, and integration overheads_

### ⏰ **Months of development** for robust error handling and retries

_Including fallbacks, monitoring, and state recovery_

### 🐛 **Unexpected downtimes** from LLM outages cascading through your stack

_One provider failure shouldn't halt everything_

### 🔄 **Thousands of lines** of custom code to maintain

_Endless debugging and updates as models evolve_

---

## How Top Teams Accelerate AI Delivery

<div class="comparison-section">

### Before Compozy: Weeks of Effort, Multiple Engineers, Complex Code

```javascript
class AIOrchestrator {
  constructor() {
    this.retryHandler = new BackoffRetry();
    this.stateStore = new DistributedStore();
    this.errorManager = new FailoverHandler();
    this.metrics = new MonitoringClient();
    this.queue = new TaskQueue();
    // ... Extensive setup code
  }

  async runWithRecovery(input) {
    const trace = this.metrics.start();
    try {
      await this.stateStore.init(input.id);
      const ctx = await this.prepareContext(input);

      for (let tryCount = 0; tryCount < 3; tryCount++) {
        try {
          const result = await this.invokePrimaryModel(ctx);
          if (!this.checkValidity(result)) {
            throw new Error("Invalid output");
          }
          return await this.finalize(result);
        } catch (err) {
          this.metrics.logError(err, { tryCount, trace });

          if (err.type === "RATE_LIMIT") {
            await this.manageRateLimit(err);
            continue;
          }

          if (tryCount === 2) {
            // Switch to backup
            try {
              const backup = await this.invokeBackupModel(ctx);
              return await this.finalize(backup);
            } catch (backupErr) {
              await this.notifyHuman(input, backupErr);
            }
          }

          await this.retryHandler.delay(tryCount);
        }
      }
    } finally {
      await this.cleanup(trace);
    }
  }

  // ... Thousands more lines for resilience and monitoring
}
```

### With Compozy: Minutes to Deploy, One Developer, YAML Simplicity

```yaml
resource: workflow
id: ai-processing
version: "1.0.0"

config:
  retries: 3
  timeout: 60s
  fallback:
    provider: anthropic
    model: claude-3-sonnet

tasks:
  - id: process
    $use: agent(local::agents.#(id=="processor"))
    action: analyze
    with:
      data: "{{ .workflow.input.data }}"
    on_error:
      - retry: exponential
      - fallback: secondary_provider
      - escalate: human_review

outputs:
  result: "{{ .tasks.process.output }}"
  trace_id: "{{ .workflow.execution_id }}"
```

</div>

---

## Compozy vs. The Competition

<table class="comparison-table">
  <thead>
    <tr>
      <th>Feature</th>
      <th>Compozy (Go)</th>
      <th>LangChain (Python)</th>
      <th>Custom Builds</th>
      <th>Zapier AI</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>Built-in Resilience</strong></td>
      <td>✅ Go + Temporal</td>
      <td>❌ Manual Setup</td>
      <td>❌ From Scratch</td>
      <td>⚠️ Basic Retries</td>
    </tr>
    <tr>
      <td><strong>Multi-Model Failover</strong></td>
      <td>✅ Seamless</td>
      <td>⚠️ Custom Code</td>
      <td>❌ Challenging</td>
      <td>❌ Locked In</td>
    </tr>
    <tr>
      <td><strong>Observability</strong></td>
      <td>✅ Integrated</td>
      <td>⚠️ Limited</td>
      <td>❌ DIY</td>
      <td>✅ UI-Only</td>
    </tr>
    <tr>
      <td><strong>State Handling</strong></td>
      <td>✅ Distributed</td>
      <td>❌ Memory-Based</td>
      <td>❌ Self-Built</td>
      <td>⚠️ Shallow</td>
    </tr>
    <tr>
      <td><strong>Error Recovery</strong></td>
      <td>✅ Replayable</td>
      <td>❌ Basic</td>
      <td>❌ Manual</td>
      <td>⚠️ Simple</td>
    </tr>
    <tr>
      <td><strong>Concurrency</strong></td>
      <td>✅ Unlimited Goroutines</td>
      <td>❌ GIL-Limited</td>
      <td>⚠️ Varies</td>
      <td>❌ Sequential</td>
    </tr>
    <tr>
      <td><strong>Time to Value</strong></td>
      <td>✅ Minutes</td>
      <td>⚠️ Days</td>
      <td>❌ Weeks</td>
      <td>✅ Hours</td>
    </tr>
    <tr>
      <td><strong>Open Source</strong></td>
      <td>✅ Flexible</td>
      <td>✅ Community</td>
      <td>✅ Yours</td>
      <td>❌ Closed</td>
    </tr>
    <tr>
      <td><strong>Enterprise Scale</strong></td>
      <td>✅ Out-of-Box</td>
      <td>⚠️ Add-Ons</td>
      <td>❌ Iterative</td>
      <td>⚠️ Caps</td>
    </tr>
  </tbody>
</table>

---

## Compozy in Action

<div class="how-it-works">

### 1. Define in YAML

Craft declarative workflows for agents and tools.

### 2. Test Locally

Run `compozy dev` for instant feedback and iteration.

### 3. Deploy & Scale

One command to cloud or self-host—auto-scales effortlessly.

</div>

---

## Production Features, Effortless Setup

### 🏗️ **Go + Temporal: Unrivaled Reliability**

Powered by the same tech stack as Netflix and Uber, handling massive parallel AI tasks with zero overhead.

### 🔄 **Intelligent Multi-Model Routing**

Auto-failover across providers like OpenAI, Anthropic, and Groq. Built-in cost optimization.

### 🛡️ **Enterprise Security**

- Isolated executions via Bun
- Encrypted secrets
- RBAC controls
- Full audits
- SOC2 compliance

### 📊 **Observability Built-In**

Trace workflows, monitor in real-time, alert proactively—no extras needed.

### 🔌 **MCP Integration**

Native support for Model Context Protocol, the open standard for secure AI-tool connections.

<p class="explainer">
<strong>MCP Explained:</strong> Like OAuth for AI, MCP standardizes secure access to data and tools across models.
</p>

---

## Get Started Fast

```bash
# Install Compozy CLI
brew install compozy/tap/compozy

# Init a project
compozy init ai-support-bot
cd ai-support-bot

# Develop with hot reload
compozy dev

# Run workflow
compozy workflow execute support-flow
```

### Sample Production Workflow

```yaml
resource: workflow
id: support-automation
version: "1.0.0"
description: "Smart ticket handling with escalation"

agents:
  - id: classifier
    config:
      provider: openai
      model: gpt-4o
      params:
        temperature: 0.1
    instructions: |
      Classify tickets by urgency and type.
      Escalate if uncertain.

  - id: responder
    config:
      provider: anthropic
      model: claude-3-5-sonnet-20240620
      fallback:
        provider: openai
        model: gpt-4o
    instructions: |
      Craft empathetic, accurate responses using KB.

tasks:
  - id: classify_ticket
    $use: agent(local::agents.#(id=="classifier"))
    action: classify
    with:
      ticket: "{{ .workflow.input.ticket }}"
    timeout: 10s

  - id: fetch_context
    $use: tool(local::tools.#(id=="kb_search"))
    with:
      query: "{{ .tasks.classify_ticket.output.category }}"
      limit: 5

  - id: route_ticket
    type: router
    condition: "{{ .tasks.classify_ticket.output.urgency }}"
    routes:
      high:
        - id: escalate_human
          $use: tool(local::tools.#(id=="urgent_ticket_create"))
          with:
            ticket: "{{ .workflow.input.ticket }}"
            classification: "{{ .tasks.classify_ticket.output }}"
      medium:
        - id: auto_respond
          $use: agent(local::agents.#(id=="responder"))
          action: generate_response
          with:
            ticket: "{{ .workflow.input.ticket }}"
            context: "{{ .tasks.fetch_context.output }}"
          review_required: true
      low:
        - id: auto_resolve
          $use: agent(local::agents.#(id=="responder"))
          action: generate_response
          with:
            ticket: "{{ .workflow.input.ticket }}"
            context: "{{ .tasks.fetch_context.output }}"

outputs:
  response: "{{ .tasks.route_ticket.selected_route.output }}"
  classification: "{{ .tasks.classify_ticket.output }}"
  execution_time: "{{ .workflow.duration_ms }}"
```

---

## Stay Ahead in the AI Race

Debugging brittle code wastes time your competitors use to innovate. Compozy lets teams deploy 10x faster, focusing on features over infrastructure.

### Execution speed wins AI battles—not just better models.

Compozy users report shipping AI capabilities quarterly, not yearly.

<div class="urgency-box">
  <h3>🚀 Join Forward-Thinking Teams in Production</h3>
  <p>Switch to Compozy now and receive:</p>
  <ul>
    <li>Expert migration support</li>
    <li>90 days priority assistance</li>
    <li>Beta access to advanced MCP tools</li>
  </ul>
  <p class="deadline">Limited offer ends December 31, 2025—spots filling fast</p>
</div>

---

## Open Source Commitment

### Community Edition

**Free for all, forever**

- ✅ Unlimited local runs
- ✅ Core orchestration
- ✅ Forum support
- ✅ Apache 2.0
- ✅ No lock-in

<a href="#get-started" class="cta-primary">Start Free →</a>

### Compozy Cloud

**Managed for scale**

- Auto-scaling clusters
- Collaborative dashboards
- SLAs & support
- Effortless deploys

<a href="#cloud-waitlist" class="cta-secondary">Join Waitlist →</a>

### Enterprise Self-Host

**Your infrastructure, full control**

- Multi-cloud/on-prem
- Unlimited scale
- Dedicated experts
- Custom extensions
- Air-gapped options

<a href="#contact-sales" class="cta-secondary">Talk to Sales →</a>

---

## Ready to Build Unbreakable AI?

Focus on innovation, not orchestration headaches.

```bash
brew install compozy/tap/compozy && compozy init ai-project
```

<p align="center" class="final-cta">
  <a href="#get-started" class="cta-primary large">Deploy in Minutes →</a>
</p>

<p align="center" class="footer-links">
  <a href="/docs">📖 Docs</a> • 
  <a href="https://github.com/compozy">⭐ GitHub</a> • 
  <a href="/community">💬 Community</a> • 
  <a href="/blog">📝 Blog</a>
</p>

---

<footer>
  <p><em>Compozy: Agentic orchestration reimagined with Go, Temporal, and community-driven innovation. Built by developers fed up with fragile AI stacks.</em></p>
  
  <p class="badges">
    <img src="/badges/soc2.svg" alt="SOC2 Compliant" />
    <img src="/badges/temporal.svg" alt="Temporal Powered" />
    <img src="/badges/cncf.svg" alt="CNCF Aligned" />
  </p>
</footer>
