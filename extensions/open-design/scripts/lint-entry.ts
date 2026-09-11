import { readFileSync } from "node:fs";
import { lintArtifact, renderFindingsForAgent } from "../linter/index";
export { lintArtifact };

if (process.argv.includes("--stdio")) {
  const inputs = JSON.parse(readFileSync(0, "utf8")) as {
    path: string;
    sha256: string;
    html: string;
  }[];
  const artifacts = inputs.map(({ path, sha256, html }) => {
    const findings = lintArtifact(html);
    const counts = { P0: 0, P1: 0, P2: 0 };
    for (const finding of findings) counts[finding.severity]++;
    const feedback = renderFindingsForAgent(findings);
    return { path, sha256, findings, counts, feedback, passed: counts.P0 === 0 };
  });
  process.stdout.write(JSON.stringify({ artifacts, passed: artifacts.every(a => a.passed) }));
}
