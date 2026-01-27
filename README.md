# project-monoco

project-monoco is a personal project where I’m building a small cloud style backend in Go to practice real world engineering patterns such as API design, authentication, storage primitives, background work, and deployment workflows.

This repo is designed as a portfolio piece with a clean structure, repeatable setup, and incremental milestones.

## Why I’m building this
I’m using project-monoco to get hands on practice with:
- Go service design such as routing, middleware, and validation
- Designing APIs and data models with real constraints
- Reliability basics such as health checks, structured logs, and error handling
- Containerized local dev and reproducible environments

## What it does (current and planned)
High level goal: a single API that manages cloud like resources in a simplified way.

Planned modules:
- Auth: API keys or JWT
- Projects and Users: basic tenancy model
- Object Storage: upload, download, list objects (S3 like but simplified)
- Jobs: submit background jobs and track status
- Secrets and Config: store per project configuration values
- Observability: logs, metrics, tracing
- CLI: optional monoco CLI for interacting with the API

## Tech Stack
- Language: Go
- API: REST (OpenAPI planned)
- DB: Postgres (planned)
- Queue and Cache: Redis (planned)
- Containers: Docker and Docker Compose (optional)


## Repo Layout

```

cmd/
api/            API entrypoint
worker/         background worker entrypoint (optional)
internal/
config/         config loading
middleware/     auth, logging, rate limit, etc
models/         domain models
services/       business logic
storage/        storage interfaces plus implementations
db/
migrations/     SQL migrations
deploy/
docker/         docker files and compose

````

## Getting Started

### Prereqs
- Go (1.22 or newer recommended)
- Docker (optional if using compose)

### Run locally
```bash
go mod tidy
go run ./cmd/api
````

If you use environment variables:

```bash
export PORT=8080
go run ./cmd/api
```

### Run with Docker (optional)

```bash
docker compose up --build
```

API should be available at:

* [http://localhost:8080](http://localhost:8080)

## Example Usage

Health check:

```bash
curl http://localhost:8080/health
```

## Roadmap

* [ ] API skeleton plus routing
* [ ] Config plus env handling
* [ ] Health checks plus structured logging
* [ ] Users and projects plus basic tenancy
* [ ] Auth (API keys or JWT)
* [ ] Storage (local FS first, S3 later)
* [ ] Background jobs plus worker
* [ ] Metrics and tracing plus dashboards
* [ ] CLI

## Notes

This is a personal learning project and may change quickly as I iterate. Issues and suggestions are welcome.