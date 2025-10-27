type ValidationInput = {
  value: number;
};

type ValidationOutput = {
  value: number;
};

export default {
  validate_input({ input }: { input: ValidationInput }): ValidationOutput {
    if (!Number.isFinite(input.value)) {
      throw new Error("value must be a finite number");
    }
    if (input.value <= 0) {
      throw new Error(`value must be positive; received ${input.value}`);
    }
    return { value: input.value };
  },
};
