import { query } from "@anthropic-ai/claude-code";
import path from "path";

interface ClaudeCodeInput {
  prompt: string;
}

interface ClaudeCodeOutput {
  success: boolean;
}

export async function claudeCode(input: ClaudeCodeInput): Promise<ClaudeCodeOutput> {
  try {
    let hasResults = false;
    // this is a workaround to avoid using our local api key
    process.env.ANTHROPIC_API_KEY = "";

    // Use Claude Code SDK for analysis
    for await (const msg of query({
      prompt: input.prompt,
      options: {
        cwd: path.join(__dirname, "..", "..", ".."),
        stderr: (data: string) => {
          // Limit stderr output to prevent buffer accumulation
          if (data.length > 100) {
            console.error(data.substring(0, 100) + "...");
          } else {
            console.error(data);
          }
        },
        model: "claude-3-5-haiku-latest",
        executable: "node", // Use Node.js instead of Bun to avoid nested Bun processes
        executableArgs: [
          "--max-old-space-size=1024", // Reduce Node.js heap to 1GB to prevent OOM
          "--max-semi-space-size=16", // Limit new space to 16MB
        ],
        // Ensure the claude executable is spawned via the provided path, not PATH lookup
        pathToClaudeCodeExecutable: "/Users/pedronauck/.claude/local/node_modules/.bin/claude",
        permissionMode: "bypassPermissions",
        // Limit parallel tool execution
        maxTurns: 10, // Limit total turns to prevent runaway execution
        mcpServers: {
          zen: {
            type: "stdio",
            command: "/Users/pedronauck/Dev/ai/zen-mcp-server/.zen_venv/bin/python",
            args: ["/Users/pedronauck/Dev/ai/zen-mcp-server/server.py"],
            env: {
              // Add Python unbuffered output to prevent stdio buffering
              PYTHONUNBUFFERED: "1",
            },
          },
        },
      },
    })) {
      if (msg.type === "assistant") {
        // Only log first 500 chars to prevent console buffer overflow
        const content = JSON.stringify(msg.message.content);
        if (content.length > 500) {
          console.log(content.substring(0, 500) + "... [truncated]");
        } else {
          console.log(content);
        }
        hasResults = true;
      }
    }

    if (!hasResults) {
      throw new Error("No analysis result generated from Claude Code");
    }

    return {
      success: true,
    };
  } catch (error: any) {
    throw new Error(`Claude Code analysis failed: ${error.message}`);
  }
}
