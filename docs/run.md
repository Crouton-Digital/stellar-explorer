# Running Crouton Stellar Explorer

Docker Compose runs the full project: PostgreSQL stores history, the indexer loads Stellar Testnet data, and the Go API serves it to the Nuxt frontend. This setup does not require Go or Node.js to be installed on your computer.

## Requirements

- Docker Engine running, with Docker Compose v2 or later.
- Internet access for the initial build and Testnet data ingestion.
- Available ports: `3000` for the UI, `8080` for the API, and `5432` for PostgreSQL. The database host port is configurable.

Run all commands below from the repository root, where `docker-compose.yml` is located.

## First startup

Create `.env` if it does not already exist. This command preserves an existing configuration:

```bash
if [ ! -f .env ]; then
    cp .env.example .env
fi
```

If PostgreSQL is already running on your computer and port `5432` is occupied, choose an available port in `.env`:

```dotenv
POSTGRES_PORT=5433
```

For example, `POSTGRES_PORT=5433` exposes PostgreSQL on host port **5433**. Keep `db:5432` in the Docker `DATABASE_URL`: containers connect to the internal database port, while `POSTGRES_PORT` changes only the port exposed on your computer.

Build the images and start the services in the background:

```bash
docker compose up --build -d
docker compose ps
```

The first build may take a few minutes. The API and indexer apply migrations automatically. Upgrading an existing history can also take several minutes while interpretations are rebuilt; the API becomes ready after the atomic upgrade commits. Its health check allows a ten-minute startup period. Keep the database volume and wait for readiness. With an empty database, the indexer starts roughly 100 ledgers behind the current network tip and continues ingesting new ledgers.

## Open the UI

- **Explorer:** http://localhost:3000
- **API overview:** http://localhost:8080/api/v1/overview
- **API readiness:** http://localhost:8080/health/ready
- **OpenAPI:** http://localhost:8080/openapi.json

The home page displays synchronization status and recent transactions. If the tables are empty, wait for the first data to arrive.

For a complete screen map, panel descriptions and control explanations, see the [UI guide](ui-guide.md).

To explore the main screens:

1. Open the home page and check the last indexed ledger and indexer lag.
2. Navigate to the ledger list, open a ledger, and then open one of its transactions.
3. Start with What happened to read supported actions and failure reasons. Applied actions are distinguished from unsuccessful attempts. Classic path-payment swaps separate actual sent/received amounts from request limits and include executed trade evidence. Supported Soroswap and Aquarius router exchanges also show the actual trader, request limits, pool route and committed event indexes. Aquarius strict receive shows net input spent separately from maximum funding and returned unused input; router settlement evidence includes funding, payout and nullable input-return event indexes. Supported split routes show each branch's weight, path, endpoint amounts and pool references. Intermediate amounts are not added to aggregate swap totals. Expand the router evidence to inspect checks and executable observations; a missing WASM hash is not code verification, and an Aquarius observation is not a pinned-code or audit claim. Aquarius pool indexes are request identifiers and pool addresses are matched from events, not independently registry-verified. Token movement actions preserve the same swap transfers. State changes shows before/after values, unknown or archived baselines, and operation provenance, with ten writes per page. The Fees and refunds panel shows a reconciled charge/refund/net breakdown when CAP-67 events are provided; missing or invalid breakdowns remain unknown. Inspect the operations, decoded Soroban calls/events, execution result and raw XDR for further details.
4. Follow an account or contract address to view its indexed history.
5. Click the header search button, press `/`, or use Ctrl/Cmd+K to open the centered search palette. Type a ledger number or at least four hash/address characters. Choose a live indexed match with the mouse or Up/Down and Enter. Escape, the close button or the blurred backdrop dismisses the palette. Full identifiers still work with the Search button; see [live search](search.md).
6. Open Transactions and filter by participant, contract, function, execution status, or ledger range. Applying a new filter starts from the newest matching page.
7. Open Events or Token movements to filter committed activity. In Events, choose Transaction events and the event name `fee` to inspect fees/refunds, including failed transactions. Copy the URL to retain the applied conditions, or use Reset filters to clear them. See [activity filters](filters.md) for the API semantics.

This is a partial Testnet index, so an arbitrary historical transaction may be absent from the local database. A [reproducible demo without RPC](../README.md#reproducible-demo-without-rpc) is available using a saved ledger; it requires a separate empty database.

## Logs and startup checks

Check container status:

```bash
docker compose ps
```

View recent API and indexer messages:

```bash
docker compose logs --tail=100 api indexer
```

Follow indexer logs in real time:

```bash
docker compose logs -f indexer
```

Press `Ctrl+C` to stop following logs. The containers continue running.

Check HTTP responses:

```bash
curl --fail http://localhost:8080/health/ready
curl --fail http://localhost:8080/api/v1/overview
curl --fail --output /dev/null http://localhost:3000
```

## Stop and restart

Stop the containers while keeping them available for the next startup:

```bash
docker compose stop
```

Start stopped containers:

```bash
docker compose start
```

Remove the containers and network while preserving PostgreSQL history:

```bash
docker compose down
```

After `down`, use `docker compose up -d`. After changing source files, rebuild the affected images:

```bash
docker compose up --build -d
```

The indexer resumes from its saved checkpoint. `docker compose down -v` also deletes the volume containing indexed history; it is not needed for a normal shutdown.

## Develop the UI with hot reload

Frontend development requires Node.js **22.21.1 or later** and npm. The database, API, and indexer can keep running in Docker. Stop only the frontend container to free port `3000`:

```bash
docker compose stop web
cd web
npm ci
NUXT_API_BASE=http://localhost:8080 npm run dev
```

Open http://localhost:3000. Vue/CSS changes appear automatically. To return to the built container, stop the development server with `Ctrl+C`, return to the repository root, and run:

```bash
cd ..
docker compose up --build -d web
```

## NFT history and activity charts

Open **NFTs** for observed collection contracts, then choose an exact token ID to inspect its movement history. Counts cover the displayed `snapshot_ledger`; filters limit the movement list only. Current ownership, supply, metadata and artwork are not inferred. NFT participants link to filtered NFT history, including contract recipients without generic metadata observations. Transactions and known account/contract pages have separate NFT movement panels. See [NFT evidence and API](nfts.md).

Overview, Transactions and Token movements display bounded activity charts over at most 120 indexed ledger rows. The count scale, scope caption and selected bucket remain visible on mobile. Use arrows or keyboard controls to inspect buckets, or expand Activity data. Outcome/movement-kind counts describe records in that window; token amounts are not summed. The event statistics API is available without adding another chart to the Events page. See [activity scope](activity.md).

## Troubleshooting

- **Port occupied:** stop your previous application instance or change the relevant port. For PostgreSQL, set `POSTGRES_PORT` in `.env`.
- **Tables remain empty:** check `docker compose logs --tail=100 indexer`. When RPC is unavailable, the indexer retries requests with a delay; stored data remains available.
- **Database password errors:** `POSTGRES_PASSWORD` must match the password in `DATABASE_URL`. Changing `.env` does not change the password of an existing PostgreSQL user.
- **`history gap` or a Testnet reset:** the saved checkpoint can no longer be continued from the available RPC history range. Use a separate new database; existing history is never deleted automatically. `START_LEDGER` does not override a saved checkpoint.

See the [README](../README.md#configuration) for environment variables and [tests/README.md](../tests/README.md) for automated test commands.
