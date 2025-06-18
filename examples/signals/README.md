# Signals Example

This example demonstrates signal communication between workflows in Compozy, showcasing how workflows can trigger and communicate with each other.

## What it tests

- **Signal-based workflow triggers** using the `signal` task type
- **Workflow communication** through named signals
- **Event-driven architecture** where one workflow triggers another
- **Cross-workflow messaging** with payload data

## Workflow structure

```
sender workflow
└── send-signal (signal)
    ├── signal_id: "workflow-ready"
    ├── payload: { message: "Hello from sender!" }
    └── triggers → receiver workflow

receiver workflow (trigger: signal "workflow-ready")
└── process-signal (basic)
    └── tool: log_tool → message: "Received: {{ .input.message | default \"manually started\" }}"
```

## Tools

- **log_tool**: Simple logging tool that records messages with timestamps

## Running

```bash
cd examples/signals
../../compozy dev
```

Then trigger the workflows via the API or UI to see signal communication in action.

### Trigger via API

See the `api.http` file in this directory for example API requests you can run directly in your editor or with tools like REST Client extensions.

## Expected behavior

1. The sender workflow executes and sends a signal named "workflow-ready"
2. The signal automatically triggers any receiver workflow listening for that signal
3. The receiver workflow processes the signal and logs the received message
4. Both workflows complete independently but are connected through the signal

This example validates that:

- Signal tasks can send named signals with payloads
- Workflows can be triggered by signals from other workflows
- Cross-workflow communication works reliably
- Signal payloads are properly transmitted and accessible
