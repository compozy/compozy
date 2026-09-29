interface KnowledgeGuard {
  title: string;
  description: string;
}

function knowledgeGuard(options: {
  requiresWorkspace: boolean;
  requiresAgentName: boolean;
}): KnowledgeGuard | null {
  if (options.requiresWorkspace) {
    return {
      title: "Open a project first",
      description: "Knowledge for a project shows up after you open one.",
    };
  }
  if (options.requiresAgentName) {
    return { title: "Choose an agent", description: "Pick an agent to see what it remembers." };
  }
  return null;
}

function knowledgeSearchInfo(matchCount: number): string {
  return `${matchCount} ${matchCount === 1 ? "match" : "matches"}`;
}

export { knowledgeGuard, knowledgeSearchInfo };
export type { KnowledgeGuard };
