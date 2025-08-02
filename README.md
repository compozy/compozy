# Compozy

<div align="center">
  <p>
    <strong>Next-level Agentic Orchestration Platform</strong>
  </p>
  <p>
    <a href="https://github.com/compozy/compozy/actions/workflows/ci.yml">
      <img src="https://github.com/compozy/compozy/actions/workflows/ci.yml/badge.svg" alt="Build Status">
    </a>
    <a href="https://goreportcard.com/report/github.com/compozy/compozy">
      <img src="https://goreportcard.com/badge/github.com/compozy/compozy" alt="Go Report Card">
    </a>
    <a href="https://github.com/compozy/compozy/blob/main/LICENSE">
      <img src="https://img.shields.io/github/license/compozy/compozy" alt="License">
    </a>
  </p>
</div>

> [!WARNING]
> This project is currently in alpha. Please use with caution, as it may contain bugs and undergo significant changes.

**Compozy** is a powerful, open-source workflow orchestration engine designed for building, deploying, and managing complex AI-powered applications. It simplifies the development of AI agents by providing a declarative YAML-based approach, allowing you to focus on your application's logic instead of boilerplate and integration complexities.

## ✨ Key Features

- **Declarative Workflows**: Define complex AI workflows with simple, human-readable YAML.
- **Developer-Focused**: A comprehensive CLI with hot-reloading for a seamless development experience.
- **Advanced Task Orchestration**: 8 powerful task types including parallel, sequential, and conditional execution.
- **Extensible Tools**: Write custom tools in TypeScript/JavaScript to extend agent capabilities.
- **Multi-Model Support**: Integrates with 7+ LLM providers like OpenAI, Anthropic, Google, and local models.
- **Enterprise-Ready**: With Temporal behind the scenes, Compozy is built for production with persistence, monitoring, and security features.
- **High Performance**: Built with Go at its core, Compozy delivers exceptional speed and efficiency.
- **Multi-Model Support**: Integrates with 7+ LLM providers like OpenAI, Anthropic, Google, and local models.

## 🚀 Getting Started

Get up and running with Compozy in just a few minutes.

```bash
# Install the Compozy CLI
brew install compozy/tap/compozy

# Create a new project
compozy init my-ai-app
cd my-ai-app

# Start the development server
compozy dev
```

For a complete walkthrough, check out our [**Quick Start Guide**](./docs/content/docs/core/getting-started/quick-start.mdx).

## 📚 Documentation

Our documentation website is the best place to find comprehensive information, tutorials, and API references.

| Section                                                                             | Description                                       |
| ----------------------------------------------------------------------------------- | ------------------------------------------------- |
| 🚀 **[Getting Started](./docs/content/docs/core/getting-started/installation.mdx)** | Installation, setup, and your first workflow.     |
| 💡 **[Core Concepts](./docs/content/docs/core/getting-started/core-concepts.mdx)**  | Understand Workflows, Agents, Tasks, and Tools.   |
| 🛠️ **[Configuration](./docs/content/docs/core/configuration/project-setup.mdx)**    | Project, runtime, and provider configuration.     |
| 🤖 **[Agents](./docs/content/docs/core/agents/overview.mdx)**                       | Building and configuring AI agents.               |
| ⚙️ **[Tasks](./docs/content/docs/core/tasks/overview.mdx)**                         | Orchestrating operations with various task types. |
| 🔌 **[API Reference](./docs/content/docs/api/overview.mdx)**                        | Programmatic access via the REST API.             |
| 💻 **[CLI Reference](./docs/content/docs/cli/overview.mdx)**                        | Full reference for the command-line interface.    |

**[➡️ Explore the full documentation](./docs/content/docs/core/index.mdx)**

## 🤝 Community & Contributing

We welcome contributions from the community! Whether it's reporting a bug, suggesting a feature, or submitting a pull request, your input is valuable.

- **[GitHub Issues](https://github.com/compozy/compozy/issues)**: Report bugs and request features.
- **[Contributing Guide](./CONTRIBUTING.md)**: Learn how to contribute to the project.

---

## 🔐 License

This project is licensed under the Business Source License 1.1 (BUSL-1.1). See the [LICENSE](LICENSE) file for details.

<p align="center">Made with ❤️ by the Compozy Team</p>
