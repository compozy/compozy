# Human-in-the-Loop Tools

Tools that require user confirmation or input.

## Overview

Human-in-the-loop tools pause execution waiting for user input. Use `status === "requires-action"` to detect this state and `submitResult` to provide the user's response.

## Confirmation Pattern

Ask user to confirm before executing:

```tsx
// Backend tool returns requires-action status
const deleteTool = tool({
  description: "Delete a file (requires user confirmation)",
  inputSchema: z.object({ path: z.string() }),
  execute: async ({ path }) => {
    // Return requires-action to wait for confirmation
    return { action: "confirm", path };
  },
});

// Frontend shows confirmation UI
const DeleteToolUI = makeAssistantToolUI({
  toolName: "delete_file",
  render: ({ args, result, status, submitResult }) => {
    // Initial state - show confirmation
    if (status === "requires-action" || !result?.confirmed) {
      return (
        <div className="p-4 bg-yellow-50 border border-yellow-200 rounded-lg">
          <div className="flex items-center gap-2 mb-3">
            <WarningIcon className="text-yellow-600" />
            <span className="font-medium">Confirm deletion</span>
          </div>
          <p className="mb-4">Are you sure you want to delete <code>{args.path}</code>?</p>
          <div className="flex gap-2">
            <button
              onClick={() => submitResult({ confirmed: true })}
              className="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600"
            >
              Delete
            </button>
            <button
              onClick={() => submitResult({ confirmed: false })}
              className="px-4 py-2 bg-gray-200 rounded hover:bg-gray-300"
            >
              Cancel
            </button>
          </div>
        </div>
      );
    }

    // After user responds
    if (result?.confirmed) {
      return (
        <div className="p-4 bg-green-50 rounded-lg">
          <CheckIcon className="text-green-500" />
          <span>File deleted: {args.path}</span>
        </div>
      );
    }

    return (
      <div className="p-4 bg-gray-50 rounded-lg">
        <span>Deletion cancelled</span>
      </div>
    );
  },
});
```

## Selection Pattern

Let user choose from options:

```tsx
const SelectToolUI = makeAssistantToolUI({
  toolName: "select_option",
  render: ({ args, result, status, submitResult }) => {
    if (status !== "complete") {
      return (
        <div className="p-4 bg-blue-50 rounded-lg">
          <p className="mb-3">{args.prompt}</p>
          <div className="flex flex-wrap gap-2">
            {args.options.map((option: any) => (
              <button
                key={option.id}
                onClick={() => submitResult({ selected: option.id })}
                className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
      );
    }

    return (
      <div className="p-4 bg-gray-50 rounded-lg">
        Selected: {args.options.find((o: any) => o.id === result?.selected)?.label}
      </div>
    );
  },
});
```

## Form Input Pattern

Collect structured data from user:

```tsx
const FormToolUI = makeAssistantToolUI({
  toolName: "collect_info",
  render: ({ args, status, submitResult }) => {
    const [formData, setFormData] = useState({});

    if (status !== "complete") {
      return (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            submitResult(formData);
          }}
          className="p-4 bg-gray-50 rounded-lg space-y-4"
        >
          <h3 className="font-medium">{args.title}</h3>

          {args.fields.map((field: any) => (
            <div key={field.name}>
              <label className="block text-sm font-medium mb-1">
                {field.label}
              </label>
              <input
                type={field.type || "text"}
                required={field.required}
                onChange={(e) =>
                  setFormData((d) => ({ ...d, [field.name]: e.target.value }))
                }
                className="w-full border rounded px-3 py-2"
              />
            </div>
          ))}

          <button
            type="submit"
            className="px-4 py-2 bg-blue-500 text-white rounded"
          >
            Submit
          </button>
        </form>
      );
    }

    return <div>Information collected</div>;
  },
});
```

## Multi-Step Workflow

Chain multiple interactions:

```tsx
const WizardToolUI = makeAssistantToolUI({
  toolName: "setup_wizard",
  render: ({ args, result, status, submitResult }) => {
    const [step, setStep] = useState(0);
    const [data, setData] = useState({});

    const steps = args.steps || [];
    const currentStep = steps[step];

    if (status === "complete") {
      return <div>Setup complete!</div>;
    }

    return (
      <div className="p-4 bg-gray-50 rounded-lg">
        <div className="mb-4">
          <div className="text-sm text-gray-500">
            Step {step + 1} of {steps.length}
          </div>
          <h3 className="font-medium">{currentStep.title}</h3>
        </div>

        <div className="mb-4">
          {currentStep.type === "select" && (
            <div className="space-y-2">
              {currentStep.options.map((opt: any) => (
                <button
                  key={opt.value}
                  onClick={() => {
                    const newData = { ...data, [currentStep.name]: opt.value };
                    setData(newData);

                    if (step < steps.length - 1) {
                      setStep(step + 1);
                    } else {
                      submitResult(newData);
                    }
                  }}
                  className="w-full p-3 text-left border rounded hover:bg-gray-100"
                >
                  {opt.label}
                </button>
              ))}
            </div>
          )}
        </div>

        {step > 0 && (
          <button
            onClick={() => setStep(step - 1)}
            className="text-blue-500"
          >
            ← Back
          </button>
        )}
      </div>
    );
  },
});
```

## Rating/Feedback Pattern

```tsx
const RatingToolUI = makeAssistantToolUI({
  toolName: "request_rating",
  render: ({ args, status, submitResult }) => {
    const [rating, setRating] = useState(0);
    const [comment, setComment] = useState("");

    if (status === "complete") {
      return <div>Thank you for your feedback!</div>;
    }

    return (
      <div className="p-4 bg-gray-50 rounded-lg">
        <p className="mb-3">{args.prompt}</p>

        <div className="flex gap-1 mb-3">
          {[1, 2, 3, 4, 5].map((star) => (
            <button
              key={star}
              onClick={() => setRating(star)}
              className={`text-2xl ${
                star <= rating ? "text-yellow-400" : "text-gray-300"
              }`}
            >
              ★
            </button>
          ))}
        </div>

        <textarea
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          placeholder="Additional comments (optional)"
          className="w-full border rounded p-2 mb-3"
        />

        <button
          onClick={() => submitResult({ rating, comment })}
          disabled={rating === 0}
          className="px-4 py-2 bg-blue-500 text-white rounded disabled:opacity-50"
        >
          Submit
        </button>
      </div>
    );
  },
});
```

## Timeout/Auto-Cancel

```tsx
const TimedToolUI = makeAssistantToolUI({
  toolName: "timed_action",
  render: ({ args, status, submitResult }) => {
    const [timeLeft, setTimeLeft] = useState(args.timeout || 30);

    useEffect(() => {
      if (status !== "requires-action") return;

      const timer = setInterval(() => {
        setTimeLeft((t) => {
          if (t <= 1) {
            submitResult({ timeout: true });
            return 0;
          }
          return t - 1;
        });
      }, 1000);

      return () => clearInterval(timer);
    }, [status, submitResult]);

    if (status !== "requires-action") {
      return <div>Action completed</div>;
    }

    return (
      <div className="p-4 bg-yellow-50 rounded-lg">
        <p>{args.message}</p>
        <p className="text-sm text-gray-500">
          Auto-cancelling in {timeLeft}s
        </p>
        <button
          onClick={() => submitResult({ confirmed: true })}
          className="mt-2 px-4 py-2 bg-blue-500 text-white rounded"
        >
          Confirm
        </button>
      </div>
    );
  },
});
```
