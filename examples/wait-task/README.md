# Wait Task Example

This example demonstrates the wait task functionality in Compozy. The wait task allows workflows to pause execution until a specific condition is met based on incoming signals.

## How it works

The workflow follows this sequence:

1. **Initial Step** (`initial_step`): Echoes a startup message
2. **Wait Task** (`wait_for_ready`): Waits for a signal where `signal.status == "ready"`
3. **Signal Processing**: When a signal is received, it processes it with the echo tool
4. **Final Step** (`final_step`): Executes after the wait condition is satisfied

## Wait Task Configuration

The wait task requires these key fields:

- `wait_for`: The signal name to listen for (e.g., "ready_signal")
- `condition`: CEL expression to evaluate signals (e.g., `signal.status == "ready"`)
- `timeout`: How long to wait before timing out (e.g., "5m")
- `processor`: Optional task to process signals before condition evaluation
- `on_success`: Where to go when condition is met

## Running the example

```bash
# Start the development server
cd examples/wait-task
make dev
```

## Testing the workflow

1. **Start the workflow:**

   ```bash
   curl -X POST http://localhost:8080/api/v1/workflows/wait-task/trigger \
     -H "Content-Type: application/json" \
     -d '{
       "input": {
         "initial_message": "Starting wait task demo",
         "wait_condition": "signal.status == \"ready\"",
         "signal_name": "ready_signal"
       }
     }'
   ```

2. **Monitor workflow status:**

   ```bash
   curl http://localhost:8080/api/v1/workflows/{workflow_execution_id}/status
   ```

3. **Send the signal to trigger continuation:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/workflows/{workflow_execution_id}/signals/ready_signal \
     -H "Content-Type: application/json" \
     -d '{
       "payload": {
         "status": "ready",
         "message": "System is ready to proceed"
       }
     }'
   ```

## Expected Flow

1. Workflow starts and executes `initial_step`
2. Workflow transitions to `wait_for_ready` and waits for signals
3. When you send a signal with `status: "ready"`, the condition is satisfied
4. Workflow proceeds to `final_step` and completes

## Key concepts

- **Sequential Execution**: Tasks connected via `on_success` transitions
- **Signal Waiting**: `wait_for` specifies which signal to listen for
- **Condition Evaluation**: Uses CEL expressions to evaluate signal data
- **Signal Processing**: Optional processor that runs when signals are received
- **Timeout Handling**: Prevents infinite waiting with configurable timeouts
- **Workflow Continuation**: Automatic continuation once the condition is met

## Troubleshooting

- **Workflow completes immediately**: Check that tasks have `on_success` transitions
- **Wait task doesn't wait**: Ensure `wait_for` field is specified
- **Condition never satisfied**: Verify signal payload structure matches condition
- **Timeout errors**: Increase timeout value or check signal sending
