# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
## Unreleased

### ♻️  Refactoring

- *(cli)* Improve init command
- *(docs)* Disable theme switcher
- *(llm)* General improvements ([#48](https://github.com/compozy/compozy/issues/48))
- *(parser)* Add cwd as struct on common
- *(parser)* Change from package_ref to use
- *(parser)* Improve errors
- *(parser)* Add config interface
- *(parser)* Add validator interface
- *(parser)* Improve validator
- *(parser)* Change from package_ref to pkgref
- *(parser)* Add schema validator in a package
- *(parser)* Remove ByRef finders on WorkflowConfig
- *(parser)* Add schema in a separate package
- *(parser)* Use a library to load env
- *(parser)* Remove parser.go file
- *(parser)* Adjust errors
- *(repo)* General improvements
- *(repo)* Apply go-blueprint architecture
- *(repo)* Change testutils to utils
- *(repo)* Types improvements
- *(repo)* General improvements
- *(repo)* General adjustments
- *(repo)* Build improvements
- *(repo)* Change architecture
- *(repo)* Small improvements
- *(repo)* Adjust architecture
- *(repo)* Improve protobuf integration
- *(repo)* Change to DDD
- *(repo)* Change from trigger to opts on workflow.Config
- *(repo)* Adapt to use new pkgref
- *(repo)* Change from orchestrator to worker
- *(repo)* Complete change task state and parallel execution ([#16](https://github.com/compozy/compozy/issues/16))
- *(repo)* Use Redis for store configs ([#24](https://github.com/compozy/compozy/issues/24))
- *(repo)* General improvements
- *(repo)* Config global adjustments
- *(repo)* Improve test suite
- *(worker)* Remove PAUSE/RESUME for now
- *(worker)* Create worker.Manager
- *(worker)* Avoid pass task.Config through activities ([#31](https://github.com/compozy/compozy/issues/31))
- *(worker)* Split task executors ([#77](https://github.com/compozy/compozy/issues/77))

### ⚡ Performance Improvements

- *(repo)* Improve startup performance ([#74](https://github.com/compozy/compozy/issues/74))

### 🎉 Features

- *(cli)* Add watch flag on dev
- *(core)* Add basic core structure
- *(nats)* Add first NATS server integration
- *(parser)* Add models and provider
- *(parser)* Add EnvMap methods
- *(parser)* Add LoadId method on Config
- *(parser)* Add WithParamsValidator
- *(pb)* Add ToSubject() method for events
- *(ref)* Add inline merge directive
- *(repo)* Add schema generation
- *(repo)* Run server using workflows on dev cmd
- *(repo)* Implement better log using CharmLog
- *(repo)* Add support for LogMessage on NATS
- *(repo)* Adding file references
- *(repo)* Add initial file ref loaders
- *(repo)* Adad tplengine package
- *(repo)* Add Deno runtime integration ([#1](https://github.com/compozy/compozy/issues/1))
- *(repo)* Add protobuf integration
- *(repo)* Add initial orchestrator logic ([#3](https://github.com/compozy/compozy/issues/3))
- *(repo)* Add UpdateFromEvent on states
- *(repo)* Handle workflow execute
- *(repo)* Init task.Executor
- *(repo)* Return full state on executions route
- *(repo)* Add version on API and events
- *(repo)* Use SQLite for store
- *(repo)* Add workflow and task routes
- *(repo)* Add agent definitions routes
- *(repo)* Add tools routes
- *(repo)* Integrate Swagger
- *(repo)* Add new pkg/ref ([#7](https://github.com/compozy/compozy/issues/7))
- *(repo)* Add initial temporal integration ([#8](https://github.com/compozy/compozy/issues/8))
- *(repo)* Normalize task state
- *(repo)* Add basic agent execution ([#9](https://github.com/compozy/compozy/issues/9))
- *(repo)* Implement tool call within agent ([#10](https://github.com/compozy/compozy/issues/10))
- *(repo)* Implement tool call within task
- *(repo)* Implement parallel execution for tasks ([#12](https://github.com/compozy/compozy/issues/12))
- *(repo)* Implement router task
- *(repo)* Implement collection tasks ([#17](https://github.com/compozy/compozy/issues/17))
- *(repo)* Adding MCP integration ([#25](https://github.com/compozy/compozy/issues/25))
- *(repo)* Support sequential mode for collection tasks ([#36](https://github.com/compozy/compozy/issues/36))
- *(repo)* Implement auto load for resources ([#37](https://github.com/compozy/compozy/issues/37))
- *(repo)* Implement aggregate task type ([#38](https://github.com/compozy/compozy/issues/38))
- *(repo)* Add composite task type ([#47](https://github.com/compozy/compozy/issues/47))
- *(repo)* Add signals for workflows ([#51](https://github.com/compozy/compozy/issues/51))
- *(repo)* Add nested collection tasks ([#55](https://github.com/compozy/compozy/issues/55))
- *(repo)* Add basic monitoring system ([#58](https://github.com/compozy/compozy/issues/58))
- *(repo)* Add outputs for workflow ([#76](https://github.com/compozy/compozy/issues/76))
- *(repo)* Add scheduled workflows ([#98](https://github.com/compozy/compozy/issues/98))
- *(repo)* Add task type wait ([#100](https://github.com/compozy/compozy/issues/100))
- *(repo)* Add memory ([#104](https://github.com/compozy/compozy/issues/104))
- *(repo)* Add rest api for memory ([#108](https://github.com/compozy/compozy/issues/108))
- *(repo)* Add BunJS as runtime ([#114](https://github.com/compozy/compozy/issues/114))
- *(repo)* Add task engine refac ([#116](https://github.com/compozy/compozy/issues/116))
- *(repo)* Add pkg/config ([#124](https://github.com/compozy/compozy/issues/124))
- *(repo)* Add authsystem ([#133](https://github.com/compozy/compozy/issues/133))
- *(repo)* Add missing CLI commands ([#137](https://github.com/compozy/compozy/issues/137))
- *(repo)* Add default tools ([#138](https://github.com/compozy/compozy/issues/138))
- *(repo)* Move cache and redis config to pkg/config ([#139](https://github.com/compozy/compozy/issues/139))
- *(repo)* Refactor CLI template generation
- *(server)* Add first version of the server
- *(server)* Add new route handlers
- *(task)* Add outputs to task
- First package files
### 🐛 Bug Fixes

- *(cli)* Add missing check on init command
- *(docs)* Hero text animation
- *(docs)* Class merge SSR
- *(docs)* Metadata og url
- *(engine)* Env file location
- *(memory)* Memory API request ([#121](https://github.com/compozy/compozy/issues/121))
- *(repo)* General improvements and fixes
- *(repo)* Adjust validations inside parser
- *(repo)* Make dev command work
- *(repo)* Adjust state type assertion
- *(repo)* Validate workflow params on Trigger
- *(repo)* Collection task ([#18](https://github.com/compozy/compozy/issues/18))
- *(repo)* Nested types of tasks
- *(repo)* Concurrency issues with logger ([#61](https://github.com/compozy/compozy/issues/61))
- *(repo)* Collection state creation ([#63](https://github.com/compozy/compozy/issues/63))
- *(repo)* Closing dispatchers ([#81](https://github.com/compozy/compozy/issues/81))
- *(repo)* Memory task integration ([#123](https://github.com/compozy/compozy/issues/123))
- *(repo)* Auth integration
- *(repo)* MCP command release
- *(repo)* General fixes
- *(repo)* Add automatic migration
- *(repo)* Broken links
- *(runtime)* Deno improvements and fixes ([#79](https://github.com/compozy/compozy/issues/79))

### 📚 Documentation

- *(repo)* Engine specs ([#2](https://github.com/compozy/compozy/issues/2))
- *(repo)* Cleaning docs
- *(repo)* Update weather agent
- *(repo)* Improve agentic process
- *(repo)* Add memory PRD
- *(repo)* Add initial multitenant PRD
- *(repo)* Rename schedule PRD
- *(repo)* Improve agentic process
- *(repo)* Add basic documentation and doc webapp ([#130](https://github.com/compozy/compozy/issues/130))
- *(repo)* General doc improvements
- *(repo)* Add OpenAPI docs
- *(repo)* Improve current docs
- *(repo)* Enhance tools docs
- *(repo)* Enhance memory docs
- *(repo)* Finish/enhance MCP documentation ([#135](https://github.com/compozy/compozy/issues/135))
- *(repo)* Remove old prds
- *(repo)* Improve documentation
- *(repo)* Add schemas on git
- *(repo)* Add vercel analytics
- *(repo)* Add readme and contributing
- *(repo)* Finish main landing page
- *(repo)* Fix navigation link
- *(repo)* Adjust text on lp
- *(repo)* Add logo on README
- *(repo)* Adjust install page
- *(repo)* Update readme

### 📦 Build System

- *(repo)* Fix golint errors
- *(repo)* Add initial makefile
- *(repo)* Lint errors
- *(repo)* Add weather example folder
- *(repo)* Fix lint errors
- *(repo)* Adjust lint warnigs
- *(repo)* Fix lint warnings
- *(repo)* Add precommit
- *(repo)* Add AI rules
- *(repo)* Add monitoring PRD
- *(repo)* Remove .vscode from gitignore
- *(repo)* Add Github Actions integrations ([#136](https://github.com/compozy/compozy/issues/136))
- *(repo)* Cleanup
- *(repo)* Format fix
- *(repo)* Improve release process
- *(repo)* Update deps
- *(repo)* Release process as Go package
- *(repo)* Rollback on release

### 🔧 CI/CD

- *(release)* Releasing new version v0.0.4 ([#162](https://github.com/compozy/compozy/issues/162))
- *(repo)* Fix actions
- *(repo)* Fix services setup
- *(repo)* Fix release
- *(repo)* Adjust versions
- *(repo)* Fix release process
- *(repo)* Fix ci
- *(repo)* Fix goreleaser
- *(repo)* Fix validate title step
- *(repo)* Fix quality action

### 🧪 Testing

- *(parser)* Add tests for agents
- *(parser)* Add tests for package ref
- *(parser)* Add tests for tools
- *(parser)* Add tests for tasks
- *(parser)* Add tests for workflow
- *(repo)* Refactor test style
- *(repo)* Add integration tests for states
- *(repo)* Fix nats tests
- *(repo)* Add store integration tests
- *(repo)* Adjust repository tests
- *(repo)* Test routes
- *(repo)* Test improvements
- *(repo)* Add basic tasks integration tests ([#57](https://github.com/compozy/compozy/issues/57))
- *(repo)* Fix integrations tests ([#59](https://github.com/compozy/compozy/issues/59))
- *(repo)* Fix testcontainer timeouts
- *(server)* Add basic tests for server

<!-- generated by git-cliff -->