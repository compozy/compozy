function requiredEnvironment(
  environment: Readonly<Record<string, string | undefined>>,
  name: string
): string {
  const value = environment[name]?.trim();
  if (!value) throw new Error(`macOS release requires ${name}.`);
  return value;
}

export function appleIDNotaryArguments(
  environment: Readonly<Record<string, string | undefined>>
): readonly string[] {
  return [
    "--apple-id",
    requiredEnvironment(environment, "APPLE_ID"),
    "--password",
    requiredEnvironment(environment, "APPLE_APP_SPECIFIC_PASSWORD"),
    "--team-id",
    requiredEnvironment(environment, "APPLE_TEAM_ID"),
  ];
}
