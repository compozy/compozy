import type { LoopContract } from "../types";

const TEMPLATE_TOKEN = /\{\{([\s\S]*?)\}\}/g;
const DIRECT_INPUT_REFERENCE = /^\s*\.inputs\.([A-Za-z_][A-Za-z0-9_]*)\s*$/;

function renderFixtureValue(value: string, inputs: Readonly<Record<string, unknown>>): string {
  return value.replace(TEMPLATE_TOKEN, (_template, expression: string) => {
    const match = expression.match(DIRECT_INPUT_REFERENCE);
    if (!match) throw new Error(`Unsupported Loop contract fixture template: {{${expression}}}`);
    const name = match[1];
    if (!Object.hasOwn(inputs, name))
      throw new Error(`Missing Loop contract fixture input: ${name}`);
    return String(inputs[name]);
  });
}

/** Materializes the direct input references used by Loop fixtures in exactly one pass. */
export function materializeContractFixture(
  contract: LoopContract,
  inputs: Readonly<Record<string, unknown>>
): LoopContract {
  return {
    ...contract,
    goal: renderFixtureValue(contract.goal, inputs),
    definition_of_done: renderFixtureValue(contract.definition_of_done, inputs),
    boundaries: contract.boundaries?.map(value => renderFixtureValue(value, inputs)),
    constraints: contract.constraints?.map(value => renderFixtureValue(value, inputs)),
  };
}
