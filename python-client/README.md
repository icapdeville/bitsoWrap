# Python Client for Bitso Go API Wrapper

Periodic data-sync service that queries the [Bitso Go API wrapper](../README.md)
and stores results in MongoDB using bulk upsert operations.

## Project structure

```
python-client/
├── bitso_client.py   # HTTP client for the Go wrapper
├── mongo_handler.py  # MongoDB bulk write operations
├── scheduler.py      # APScheduler periodic execution
├── main.py           # Entry point
├── requirements.txt  # Python dependencies
├── .env.example      # Environment variable template
├── Dockerfile        # Container definition
└── README.md
```

## Configuration

All configuration is done through environment variables.
Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

| Variable | Description | Default |
|---|---|---|
| `GO_API_URL` | URL of the running Go wrapper | `http://localhost:8181` |
| `<USER>_API_KEY` | Bitso API key for user `<USER>` | – |
| `<USER>_API_SECRET` | Bitso API secret for user `<USER>` | – |
| `BITSO_USERS` | Comma-separated user identifiers | – |
| `BITSO_BOOKS` | Comma-separated trading books | `btc_mxn` |
| `MONGODB_URI` | MongoDB connection string | – |
| `MONGODB_DB` | MongoDB database name | – |
| `SCHEDULER_INTERVAL_MINUTES` | Sync interval in minutes | `5` |

### Credential naming

Credentials are looked up using the **uppercase** user identifier as a prefix:

```bash
# For user "alice"
ALICE_API_KEY=my_key
ALICE_API_SECRET=my_secret

# For user "bob"
BOB_API_KEY=...
BOB_API_SECRET=...
```

This avoids storing credentials in YAML files on disk.

## Running locally

```bash
# Install dependencies
pip install -r requirements.txt

# Configure environment
cp .env.example .env
# edit .env with your credentials

# Run
python main.py
```

## Running with Docker

The Python client is designed to run alongside the Go wrapper using
Docker Compose. Add the following service to `compose.yaml`:

```yaml
services:
  bitso-wrapper:
    # ... existing Go service ...

  bitso-python-client:
    build: ./python-client
    depends_on:
      - bitso-wrapper
    environment:
      - GO_API_URL=http://bitso-wrapper:8080
      - BITSO_USERS=alice
      - BITSO_BOOKS=btc_mxn,eth_mxn
      - ALICE_API_KEY=${ALICE_API_KEY}
      - ALICE_API_SECRET=${ALICE_API_SECRET}
      - MONGODB_URI=${MONGODB_URI}
      - MONGODB_DB=bitso
      - SCHEDULER_INTERVAL_MINUTES=5
    restart: unless-stopped
```

> **Note:** Inside the compose network, the Go wrapper is reachable at
> `http://bitso-wrapper:8080` (the internal port), not 8181.

## MongoDB collections

| Collection | Key fields | Description |
|---|---|---|
| `tickers` | `book`, `created_at` | Price snapshots per book |
| `balances` | `user`, `currency` | Account balance per currency |
| `open_orders` | `oid` | Pending orders |
| `trades` | `tid` | Executed trades |

All documents receive a `fetched_at` timestamp on every upsert.

## Endpoints consumed from the Go wrapper

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/ticker` | GET | No | Current price quotes |
| `/balance` | GET | Yes | Account balances |
| `/open_orders` | GET | Yes | Pending orders |
| `/trades` | GET | Yes | Trade history |
