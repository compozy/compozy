import { query } from "@anthropic-ai/claude-code";
import type { ContentBlock } from "@anthropic-ai/sdk/resources/messages.mjs";

interface ClaudeCodeInput {
  prompt: string;
}

interface ClaudeCodeOutput {
  result_messages: ContentBlock[];
  success: boolean;
}

export async function claudeCode(input: ClaudeCodeInput): Promise<ClaudeCodeOutput> {
  try {
    const resultMessages: ContentBlock[] = [];
    // this is a workaround to avoid using our local api key
    process.env.ANTHROPIC_API_KEY = "";

    // Use Claude Code SDK for analysis
    for await (const msg of query({
      prompt: input.prompt,
      options: {
        cwd: "/Users/pedronauck/Dev/compozy/compozy", // Adjusted path
        stderr: (data: string) => {
          console.error(data);
        },
        model: "claude-3-5-haiku-latest",
        executable: "bun",
        pathToClaudeCodeExecutable: "/Users/pedronauck/.claude/local/node_modules/.bin/claude",
        permissionMode: "bypassPermissions",
        mcpServers: {
          zen: {
            type: "stdio",
            command: "/Users/pedronauck/Dev/ai/zen-mcp-server/.zen_venv/bin/python",
            args: ["/Users/pedronauck/Dev/ai/zen-mcp-server/server.py"],
            env: {},
          },
        },
      },
    })) {
      if (msg.type === "assistant") {
        resultMessages.push(...msg.message.content);
      }
    }

    if (resultMessages.length === 0) {
      throw new Error("No analysis result generated from Claude Code");
    }

    return {
      result_messages: resultMessages,
      success: true,
    };
  } catch (error: any) {
    throw new Error(`Claude Code analysis failed: ${error.message}`);
  }
}

// Main execution
async function main() {
  const input = JSON.parse(await Bun.stdin.text());
  const result = await claudeCode(input);
  console.log(JSON.stringify(result, null, 2));
}

main().catch(error => {
  console.error(JSON.stringify({ error: error.message }, null, 2));
  process.exit(1);
});
