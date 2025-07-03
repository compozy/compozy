interface CounterInput {
    count: number;
}

interface CounterOutput {
    count_result: number[];
    total: number;
}

export function counterTool(input: CounterInput): CounterOutput {
    // Input validation
    if (!input || typeof input !== 'object') {
        throw new Error('Invalid input: input must be an object');
    }
    
    if (typeof input.count !== 'number' || !Number.isInteger(input.count)) {
        throw new Error('Invalid input: count must be an integer');
    }
    
    if (input.count < 0) {
        throw new Error('Invalid input: count must be non-negative');
    }
    
    if (input.count > 10000) {
        throw new Error('Invalid input: count must not exceed 10000 (performance limit)');
    }

    const count_result: number[] = [];
    for (let i = 1; i <= input.count; i++) {
        count_result.push(i);
    }

    return {
        count_result,
        total: input.count,
    };
}
