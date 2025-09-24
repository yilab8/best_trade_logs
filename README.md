# Best Trade Logs

Best Trade Logs is a Go web application that helps discretionary traders capture, review, and improve their trades. It provides a structured workflow for documenting trade plans, execution, post-trade analysis, and follow-up observations such as what happened 7 or 30 days after an exit.

## Features

- **Comprehensive trade capture** – record instrument, direction, entry and exit details, stop loss, target, fees, risk plan, and qualitative notes.
- **Post-trade review** – log outcome summaries, psychology observations, improvement ideas, and tag trades for later filtering.
- **Automatic metrics** – profit/loss, return percentages, R multiples, total risk, and target R are computed automatically.
- **Follow-up tracking** – capture price observations days after the exit (e.g., +7 and +30) to evaluate missed follow-through moves.
- **Unrealized tracking** – for open positions you can supply a reference close price to estimate current performance.
- **Browser-based UI** – responsive HTML UI for listing trades, editing records, and drilling into trade details.

## Running the application

### Quick start (in-memory storage)

The default build uses an in-memory repository, which is ideal for local experiments or when you want to try the UI quickly.

```bash
go run ./cmd/server
```

Open http://localhost:8080 to access the journal.

### Using MongoDB

Full persistence is enabled when compiling with the `mongodb` build tag. You need a running MongoDB instance and the official Go driver installed (run `go get go.mongodb.org/mongo-driver/mongo` in an environment with internet access).

1. Export the required environment variables:

```bash
export MONGO_URI="mongodb://localhost:27017"
export MONGO_DB="best_trade_logs"
# optional override (defaults to "trades")
export MONGO_COLLECTION="trades"
```

2. Build and run with MongoDB support:

```bash
go build -tags mongodb ./cmd/server
go run -tags mongodb ./cmd/server
```

When MongoDB is enabled the server will connect on startup and persist trades inside the configured collection.

### Configuration

- `PORT` – HTTP port (defaults to `8080`).
- `MONGO_URI`, `MONGO_DB`, `MONGO_COLLECTION` – required when running with the `mongodb` build tag.

## Testing

Run the unit test suite:

```bash
go test ./...
```

The tests cover domain calculations, repository behaviour, service workflows, and key HTTP handler logic.

## Project structure

- `cmd/server` – application entry point and repository setup logic.
- `internal/domain/trade` – core trade entity and metric calculations.
- `internal/service/trade` – orchestration logic for trade workflows.
- `internal/storage` – in-memory and MongoDB repository implementations.
- `internal/web` – HTTP handlers and view models.
- `internal/web/templates` – HTML templates embedded into the binary.

## Next steps

- Add authentication and user accounts if you need multi-user support.
- Extend filtering/search for tags, setups, or outcomes.
- Integrate market data APIs to automatically populate follow-up prices or daily closes.
- Export analytics to spreadsheets or dashboards.
