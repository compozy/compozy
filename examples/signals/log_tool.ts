interface LogInput {
    message: string;
}

interface LogOutput {
    logged: string;
    timestamp: string;
}

export function run(input: LogInput): LogOutput {
    // Return success output
    return {
        logged: input.message,
        timestamp: new Date().toISOString(),
    };
}
