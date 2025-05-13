import * as prettier from "npm:prettier";

type Result = {
  message: string;
};

type Output = {
  success: boolean;
};

export async function run(input: Result): Promise<Output> {
  const formatted = await prettier.format(JSON.stringify(input), {
    parser: "json",
  });
  await Deno.writeTextFile("results.json", formatted);
  return { success: true };
}
