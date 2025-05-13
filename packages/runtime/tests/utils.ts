export const fixturesPath = new URL("../fixtures", import.meta.url).pathname;
export const echoToolPath = `${fixturesPath}/echo_tool.ts`;
export const weatherToolPath = `${fixturesPath}/weather_tool.ts`;

export let capturedOutput: string[] = [];
const originalConsoleError = console.error;
const originalConsoleLog = console.log;

export function setupCapture() {
  capturedOutput = [];
  console.error = (message: string) => {
    capturedOutput.push(message);
  };
  console.log = (message: string) => {
    capturedOutput.push(message);
  };
}

export function restoreConsole() {
  console.error = originalConsoleError;
  console.log = originalConsoleLog;
}
