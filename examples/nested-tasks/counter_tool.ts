interface CounterInput {
    count: number;
}

interface CounterOutput {
    count_result: number[];
    total: number;
}

export function run(input: CounterInput): CounterOutput {
    const count_result: number[] = [];
    for (let i = 1; i <= input.count; i++) {
        count_result.push(i);
    }

    return {
        count_result,
        total: input.count,
    };
}
