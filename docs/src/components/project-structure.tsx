"use client";

import { File, Folder, Tree, TreeViewElement } from "./magicui/file-tree";

const defaultProjectStructure: TreeViewElement[] = [
  {
    id: "compozy",
    name: "compozy/",
    children: [
      {
        id: "engine",
        name: "engine/",
        children: [
          {
            id: "agent",
            name: "agent/",
            children: [
              { id: "agent-readme", name: "README.md" },
              { id: "agent-service", name: "service.go" },
              { id: "agent-config", name: "config.go" },
              { id: "agent-test", name: "service_test.go" },
            ],
          },
          {
            id: "task",
            name: "task/",
            children: [
              { id: "task-readme", name: "README.md" },
              { id: "task-executor", name: "executor.go" },
              { id: "task-basic", name: "basic.go" },
              { id: "task-parallel", name: "parallel.go" },
              { id: "task-collection", name: "collection.go" },
              { id: "task-router", name: "router.go" },
              { id: "task-test", name: "executor_test.go" },
            ],
          },
          {
            id: "tool",
            name: "tool/",
            children: [
              { id: "tool-readme", name: "README.md" },
              { id: "tool-manager", name: "manager.go" },
              { id: "tool-executor", name: "executor.go" },
              { id: "tool-test", name: "manager_test.go" },
            ],
          },
          {
            id: "workflow",
            name: "workflow/",
            children: [
              { id: "workflow-readme", name: "README.md" },
              { id: "workflow-engine", name: "engine.go" },
              { id: "workflow-config", name: "config.go" },
              { id: "workflow-test", name: "engine_test.go" },
            ],
          },
          {
            id: "runtime",
            name: "runtime/",
            children: [
              { id: "runtime-readme", name: "README.md" },
              { id: "runtime-bun", name: "bun.go" },
              { id: "runtime-node", name: "node.go" },
              { id: "runtime-test", name: "runtime_test.go" },
            ],
          },
          {
            id: "llm",
            name: "llm/",
            children: [
              { id: "llm-readme", name: "README.md" },
              { id: "llm-openai", name: "openai.go" },
              { id: "llm-groq", name: "groq.go" },
              { id: "llm-ollama", name: "ollama.go" },
              { id: "llm-test", name: "providers_test.go" },
            ],
          },
          {
            id: "mcp",
            name: "mcp/",
            children: [
              { id: "mcp-readme", name: "README.md" },
              { id: "mcp-client", name: "client.go" },
              { id: "mcp-transport", name: "transport.go" },
              { id: "mcp-test", name: "client_test.go" },
            ],
          },
          {
            id: "project",
            name: "project/",
            children: [
              { id: "project-readme", name: "README.md" },
              { id: "project-config", name: "config.go" },
              { id: "project-opts", name: "opts.go" },
              { id: "project-runtime", name: "runtime.go" },
              { id: "project-validators", name: "validators.go" },
              { id: "project-test", name: "config_test.go" },
            ],
          },
          {
            id: "infra",
            name: "infra/",
            children: [
              { id: "infra-readme", name: "README.md" },
              { id: "infra-server", name: "server.go" },
              { id: "infra-db", name: "db.go" },
              { id: "infra-messaging", name: "messaging.go" },
              { id: "infra-test", name: "server_test.go" },
            ],
          },
          {
            id: "core",
            name: "core/",
            children: [
              { id: "core-readme", name: "README.md" },
              { id: "core-types", name: "types.go" },
              { id: "core-error", name: "error.go" },
              { id: "core-config", name: "config.go" },
              { id: "core-test", name: "types_test.go" },
            ],
          },
        ],
      },
      {
        id: "pkg",
        name: "pkg/",
        children: [
          {
            id: "logger",
            name: "logger/",
            children: [
              { id: "logger-readme", name: "README.md" },
              { id: "logger-logger", name: "logger.go" },
              { id: "logger-test", name: "logger_test.go" },
            ],
          },
          {
            id: "mcp-proxy",
            name: "mcp-proxy/",
            children: [
              { id: "proxy-readme", name: "README.md" },
              { id: "proxy-server", name: "server.go" },
              { id: "proxy-test", name: "server_test.go" },
            ],
          },
          {
            id: "utils",
            name: "utils/",
            children: [
              { id: "utils-readme", name: "README.md" },
              { id: "utils-utils", name: "utils.go" },
              { id: "utils-test", name: "utils_test.go" },
            ],
          },
          {
            id: "tplengine",
            name: "tplengine/",
            children: [
              { id: "tpl-readme", name: "README.md" },
              { id: "tpl-engine", name: "engine.go" },
              { id: "tpl-test", name: "engine_test.go" },
            ],
          },
        ],
      },
      {
        id: "cli",
        name: "cli/",
        children: [
          { id: "cli-readme", name: "README.md" },
          { id: "cli-main", name: "main.go" },
          { id: "cli-commands", name: "commands.go" },
          { id: "cli-test", name: "cli_test.go" },
        ],
      },
      {
        id: "test",
        name: "test/",
        children: [
          {
            id: "test-helpers",
            name: "helpers/",
            children: [
              { id: "test-setup", name: "setup.go" },
              { id: "test-fixtures", name: "fixtures.go" },
            ],
          },
          {
            id: "test-integration",
            name: "integration/",
            children: [
              { id: "test-workflow", name: "workflow_test.go" },
              { id: "test-memory", name: "memory_test.go" },
            ],
          },
        ],
      },
      { id: "go-mod", name: "go.mod" },
      { id: "go-sum", name: "go.sum" },
      { id: "makefile", name: "Makefile" },
      { id: "readme", name: "README.md" },
    ],
  },
];

interface ProjectStructureProps {
  /**
   * The project structure data as a TreeViewElement array
   */
  structure?: TreeViewElement[];
  /**
   * Title to display above the tree
   */
  title?: string;
  /**
   * Initially selected item ID
   */
  initialSelectedId?: string;
  /**
   * Initially expanded items
   */
  initialExpandedItems?: string[];
  /**
   * Additional CSS classes
   */
  className?: string;
}

export default function ProjectStructure({
  title,
  structure = defaultProjectStructure,
  initialSelectedId = "project",
  initialExpandedItems = ["compozy", "engine", "project", "pkg"],
  className = "",
}: ProjectStructureProps) {
  return (
    <div
      className={`relative flex w-full flex-col items-center justify-start rounded-lg border bg-background p-4 ${className}`}
    >
      <div className="w-full max-w-4xl">
        {title && <h3 className="mb-4 text-lg font-semibold">{title}</h3>}
        <Tree
          className="w-full"
          initialSelectedId={initialSelectedId}
          initialExpandedItems={initialExpandedItems}
          elements={structure}
        >
          {structure.map(element => (
            <TreeItem key={element.id} element={element} />
          ))}
        </Tree>
      </div>
    </div>
  );
}

function TreeItem({ element }: { element: TreeViewElement }) {
  const hasChildren = element.children && element.children.length > 0;

  if (hasChildren) {
    return (
      <Folder element={element.name} value={element.id} isSelectable={element.isSelectable}>
        {element.children?.map(child => (
          <TreeItem key={child.id} element={child} />
        ))}
      </Folder>
    );
  }

  return (
    <File value={element.id} isSelectable={element.isSelectable}>
      <span>{element.name}</span>
    </File>
  );
}
