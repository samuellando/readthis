# ReadThis

ReadThis is a simple, self-hosted read-it-later tool written in Go. It stores links in a single JSON file and serves a server-rendered UI with HTMX plus a REST API.

## Run

```bash
go run ./cmd/readthis
```

By default the server listens on `:8080` and stores data at `./data/links.json`.

### Configuration

- `READTHIS_DATA_PATH` sets the data file path.
- `-addr` overrides the listen address.
- `-data` overrides the data file path.

Example:

```bash
READTHIS_DATA_PATH=./data/links.json go run ./cmd/readthis -addr :9090
```

## Tests

```bash
go test ./...
```
