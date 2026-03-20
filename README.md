# Loggos

A blockchain-based logging service written in Go. Loggos accepts log entries
over HTTP, stores them in a lightweight in-memory blockchain for tamper
detection, and optionally indexes each entry into Elasticsearch for search and
analysis.

## Features

- **Immutable log chain** — every entry is hashed and linked to the previous
  block, making tampering immediately detectable.
- **Simple HTTP API** — `GET` to read the chain, `POST` to append a log entry.
- **Optional Elasticsearch** — when configured, blocks are indexed
  automatically; when not, the service runs standalone.
- **Health endpoint** — `GET /healthz` for load balancers and orchestrators.
- **Docker support** — single-command startup with `docker compose`.

## Quick Start

### With Docker (recommended)

```bash
docker compose up --build
```

This starts Loggos on port `8080` and Elasticsearch on port `9200`.

### Without Docker

Requirements: Go 1.21+

```bash
cp .env.example .env   # edit as needed
go build -o loggos .
./loggos
```

## Configuration

All configuration is via environment variables (or a `.env` file).

| Variable            | Default     | Description                                      |
|---------------------|-------------|--------------------------------------------------|
| `ADDR`              | `8080`      | Port the HTTP server listens on                  |
| `ELASTICSEARCH_URL` | *(unset)*   | Elasticsearch URL; indexing disabled when unset   |

## API

### `GET /`

Returns the full blockchain as a JSON array.

```bash
curl http://localhost:8080/
```

### `POST /`

Appends a new block with the given log line.

```bash
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"logline":"deployment started"}'
```

### `GET /healthz`

Returns `{"status":"ok"}` with HTTP 200.

## Running Tests

```bash
go test -v -race ./...
```

## Project Structure

```
.
├── main.go              # Application source
├── main_test.go         # Unit and integration tests
├── Dockerfile           # Multi-stage container build
├── docker-compose.yml   # Full stack (Loggos + Elasticsearch)
├── .env.example         # Example environment configuration
├── .github/
│   ├── workflows/ci.yml # CI pipeline (build, test, lint)
│   ├── dependabot.yml   # Automated dependency updates
│   ├── ISSUE_TEMPLATE/  # Bug report and feature request templates
│   └── pull_request_template.md
├── CONTRIBUTING.md      # Contribution guidelines
├── SECURITY.md          # Vulnerability reporting policy
└── LICENSE              # MIT License
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md) for how to report vulnerabilities.

## License

[MIT](LICENSE)
