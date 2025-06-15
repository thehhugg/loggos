# Loggos

Loggos is a simple logging service written in Go. It accepts log entries over HTTP,
stores them in a lightweight blockchain for immutability, and indexes each
entry into Elasticsearch.

## Building

This project uses Go modules. To download dependencies and build the binary run:

```bash
go build
```

## Running

Create a `.env` file (see the included example) with the following variables:

- `ADDR` – port for the HTTP server (default `8080`)
- `ELASTICSEARCH_URL` – address of the Elasticsearch instance

Then start the service:

```bash
go run main.go
```

The server exposes two endpoints:

- `GET /` – returns the current blockchain as JSON
- `POST /` – accept a JSON body like `{"logline":"your text"}` to append a
  new block

Every valid block is also indexed into Elasticsearch under the `loggos` index.

