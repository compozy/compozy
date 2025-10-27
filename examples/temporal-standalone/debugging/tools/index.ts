type ValidationInput = {
  value: number;
};

type ValidationOutput = {
  value: number;
};

export default {
  async validate_input({ input }: { input: ValidationInput }): Promise<ValidationOutput> {
    if (input.value === undefined || input.value === null) {
      throw new Error("value is required");
    }
    if (Number.isNaN(input.value)) {
      throw new Error("value must be a number");
    }
    if (input.value <= 0) {
      throw new Error(`value must be positive; received ${input.value}`);
    }
    return { value: input.value };
  },
};
