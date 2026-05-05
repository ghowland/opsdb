# Decision 001: Go Only

## Status

Accepted.

## Context

The OpsDB tooling suite includes a schema engine, an API server, a runner framework library, core runners (change-set executor, schema executor, reaper, emergency review monitor, notification runner), and importers for seven authority types. All of these are operational software — they manage infrastructure, and they must be more reliable than the infrastructure they manage.

The spec draws a hard line between operational logic and application logic. Operational logic assumes failure, minimizes dependencies, caches locally for partition tolerance, and stays running when the environment degrades. Application logic assumes the environment works and exits cleanly when it doesn't. OpsDB tooling is operational logic.

## Decision

All OpsDB tooling is written in Go. No Python, no TypeScript, no mixed-language builds. Organizations that want Python or other language implementations write their own against the published contracts and test suites.

## Rationale

**Single static binary.** Go compiles to a single binary with no runtime dependencies. No virtualenv, no pip, no package manager at deploy time, no interpreter version conflicts, no shared library compatibility issues. Copy the binary to the target, run it. This is the minimum-dependency principle applied to the tooling itself.

**Operational logic properties.** Go produces fast-starting, low-memory processes with predictable performance characteristics. Garbage collection pauses are bounded and short. Concurrency is native and lightweight. These properties matter for long-running runners, for the API server handling concurrent requests, and for importers processing large volumes of authority data.

**One language, one build system, one dependency tree.** A monorepo with Go means one `go.mod`, one `go build`, one dependency audit surface. Mixed languages mean multiple build systems, multiple dependency trees, multiple security audit surfaces, multiple CI configurations. Every additional language multiplies operational surface area.

**Runner authoring simplicity.** The runner framework library is designed so that writing a runner in Go is straightforward — 200 lines of runner-specific logic, 50 lines of library glue. The claim that Python "makes runners easier to write" assumes that the library suite is insufficient. If the library suite is good, the language matters less than the library quality. We make the Go library good enough that "but Python is easier" doesn't hold.

**Community contributions converge.** If the project accepts runners in Go and Python, every shared pattern must be implemented twice, tested twice, documented twice, and maintained twice. Bug fixes must land in both implementations. Contract changes must be validated against both test suites. The cost scales with the number of languages. One language means one implementation of each pattern, one test suite, one documentation set.

## Tradeoffs

Organizations with no Go expertise face a learning curve. Go is not a complex language — a Python developer can write working Go within a week — but the curve exists. The project accepts this tradeoff because the operational benefits of single-binary deployment and the maintenance benefits of a single language outweigh the onboarding cost.

Organizations that have strong Python operational tooling cannot directly reuse it as OpsDB runners. They can wrap their existing Python code behind a Go runner that shells out, or they can rewrite in Go using the library suite. The project provides the contracts and test suites so that a community-maintained Python implementation can exist, but the project itself does not maintain one.
