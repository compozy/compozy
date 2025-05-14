// deno-lint-ignore require-await
export async function run(context: { message: string }) {
  return {
    echo: context.message,
  };
}
