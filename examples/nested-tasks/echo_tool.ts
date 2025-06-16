interface EchoInput {
    message: string;
}

interface EchoOutput {
    echoed_message: string;
    timestamp: string;
}

export function run(input: EchoInput): EchoOutput {
    return {
        echoed_message: input.message,
        timestamp: new Date().toISOString(),
    };
}
