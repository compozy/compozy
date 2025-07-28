# Contributing to Compozy

First off, thank you for considering contributing to Compozy! It's people like you that make Compozy such a great tool. We welcome contributions from the community, whether it's reporting a bug, suggesting a feature, or submitting a pull request.

## Table of Contents

- [Contributing to Compozy](#contributing-to-compozy)
  - [Table of Contents](#table-of-contents)
  - [Code of Conduct](#code-of-conduct)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
  - [Development Workflow](#development-workflow)
    - [Running the Development Server](#running-the-development-server)
    - [Running Tests](#running-tests)
    - [Linting and Formatting](#linting-and-formatting)
    - [Commit Message Guidelines](#commit-message-guidelines)
  - [Submitting Changes](#submitting-changes)
    - [Pull Request Process](#pull-request-process)
  - [Coding Standards](#coding-standards)
  - [License](#license)

## Code of Conduct

This project and everyone participating in it is governed by the [Compozy Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) (version 1.24 or later)
- [Bun](https://bun.sh/docs/installation)
- [Docker](https://docs.docker.com/get-docker/)
- [Make](https://www.gnu.org/software/make/)

### Installation

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally:
    ```bash
    git clone https://github.com/YOUR_USERNAME/compozy.git
    cd compozy
    ```
3.  **Install dependencies**:
    ```bash
    make deps
    ```
4.  **Set up the database and other services**:
    ```bash
    make start-docker
    make migrate-up
    ```

## Development Workflow

### Running the Development Server

First, you need to set up the environment variables. Copy the `.env.example` file to `.env` and fill in the values.

```bash
cp .env.example .env
```

To start the development server with hot-reloading, you can run any example workflow with:

```bash
make dev EXAMPLE=[example-name] # example name is the folder name under the examples/ directory
```

This will start the Compozy server and watch for changes in the source code.

### Running Tests

To run the test suite, use the following command:

```bash
make test
```

This command runs both Go and Bun tests.

### Linting and Formatting

Before committing your changes, make sure to lint and format your code:

```bash
make fmt
make lint
```

This will format both Go and TypeScript/JavaScript code and run the linters.

### Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification. This allows for automated changelog generation and helps keep the commit history clean and readable.

**Format**:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Example**:

```
feat(engine): add support for parallel tasks
```

## Submitting Changes

### Pull Request Process

1.  Ensure that your code adheres to the project's coding standards and that all tests pass.
2.  Create a new pull request from your fork to the `main` branch of the Compozy repository.
3.  Provide a clear and descriptive title and description for your pull request.
4.  The team will review your pull request and may suggest changes.
5.  Once your pull request is approved, it will be merged into the `main` branch.

## Coding Standards

We have a set of coding standards that we follow to ensure code quality and consistency. These are documented in the `.cursor/rules` directory. Please review these documents before contributing.

## License

By contributing to Compozy, you agree that your contributions will be licensed under its [Business Source License 1.1 (BUSL-1.1)](LICENSE).
