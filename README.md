# Lipila Mock Payment Gateway

A self-contained mock server that simulates the [Lipila](https://lipila.co.zm) payment API. Built for local development and integration testing — no external dependencies, no internet required. Ships as a single `.exe` with an embedded SQLite database.

## Features

- **Collections** — Mobile money (MTN, Airtel, Zamtel) and card payments
- **Disbursements** — Mobile money and bank transfers with fee calculation
- **Async processing** — Configurable delays, random failures, and callback delivery with retry
- **Admin dashboard** — Create wallets, view transactions, tune success rates — all from the browser
- **Zero config** — Run the binary and start testing immediately

## Quick Start

### Run from source

```bash
# Requires Go 1.22+ and a C compiler (CGO needed for SQLite)
go run .
```

### Build a Windows executable

```bash
# On Windows with MinGW/GCC installed
go build -ldflags="-s -w" -o lipila-mock.exe .

# Cross-compile from Linux/macOS
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -ldflags="-s -w" -o lipila-mock.exe .
```

### Build for Linux / macOS

```bash
# Linux
CGO_ENABLED=1 go build -ldflags="-s -w" -o lipila-mock .

# macOS
CGO_ENABLED=1 go build -ldflags="-s -w" -o lipila-mock .
```

### Run the binary

```bash
# Just run it — uses defaults (port 8080, lipila.db in current directory)
./lipila-mock.exe

# Or with flags
./lipila-mock.exe -addr :9090 -db ./data/test.db
```

On startup the server prints:

```
opening database: lipila.db
database initialized
new test wallet created — API key: Lsk_abcdef1234567890abcdef1234567890
────────────────────────────────────────
  Lipila Mock Payment Gateway
  Server:   http://localhost:8080/
  Admin UI: http://localhost:8080/admin/
  API base: http://localhost:8080/api/v1/
  Health:   http://localhost:8080/health
────────────────────────────────────────
```

Copy the API key from the startup log — you'll need it for all API requests.

## URLs

| URL | Description |
|-----|-------------|
| `http://localhost:8080/` | Redirects to admin dashboard |
| `http://localhost:8080/admin/` | Admin dashboard |
| `http://localhost:8080/admin/config` | Simulation config editor |
| `http://localhost:8080/admin/wallets/new` | Create new wallet form |
| `http://localhost:8080/api/v1/` | API base URL |
| `http://localhost:8080/health` | Health check |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `lipila.db` | Path to SQLite database file |

Environment variables override command-line flags:

```bash
PORT=3000 DB_PATH=./test.db ./lipila-mock.exe
```

On Windows (PowerShell):

```powershell
$env:PORT="3000"; $env:DB_PATH="./test.db"; .\lipila-mock.exe
```

On Windows (CMD):

```cmd
set PORT=3000 && set DB_PATH=./test.db && lipila-mock.exe
```

## API Reference

All API endpoints require the `x-api-key` header. The API key is shown in the startup log and on the admin dashboard.

### Collections

#### Mobile Money Collection

```bash
curl -X POST http://localhost:8080/api/v1/collections/mobile-money \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "referenceId": "order-001",
    "amount": 100.00,
    "accountNumber": "260971234567",
    "currency": "ZMW",
    "narration": "Payment for order 001",
    "callbackUrl": "https://your-app.com/webhook"
  }'
```

#### Card Collection

```bash
curl -X POST http://localhost:8080/api/v1/collections/card \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "customerInfo": {
      "firstName": "John",
      "lastName": "Doe",
      "phoneNumber": "260971234567",
      "email": "john@example.com",
      "city": "Lusaka",
      "country": "ZM",
      "address": "123 Cairo Road",
      "zip": "10101"
    },
    "collectionRequest": {
      "referenceId": "card-001",
      "amount": 250.00,
      "accountNumber": "260971234567",
      "currency": "ZMW",
      "backUrl": "https://your-app.com/back",
      "redirectUrl": "https://your-app.com/success",
      "narration": "Card payment"
    }
  }'
```

#### Check Collection Status

```bash
curl -X GET "http://localhost:8080/api/v1/collections/check-status?referenceId=YOUR_REFERENCE_ID" \
  -H "x-api-key: YOUR_API_KEY"
```

### Disbursements

#### Mobile Money Disbursement

```bash
curl -X POST http://localhost:8080/api/v1/disbursements/mobile-money \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "referenceId": "payout-001",
    "amount": 50.00,
    "accountNumber": "260971234567",
    "currency": "ZMW",
    "narration": "Salary payment",
    "callbackUrl": "https://your-app.com/webhook"
  }'
```

> Fee: 1.5% deducted from wallet balance (amount + fee)

#### Bank Disbursement

```bash
curl -X POST http://localhost:8080/api/v1/disbursements/bank \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "referenceId": "bank-001",
    "amount": 1000.00,
    "currency": "ZMW",
    "narration": "Vendor payment",
    "accountNumber": "1234567890",
    "swiftCode": "ABORZMLU",
    "firstName": "Jane",
    "lastName": "Doe",
    "accountHolderName": "Jane Doe",
    "phoneNumber": "260971234567",
    "email": "jane@example.com",
    "callbackUrl": "https://your-app.com/webhook"
  }'
```

> Fee: 2.5% deducted from wallet balance (amount + fee)

#### Check Disbursement Status

```bash
curl -X GET "http://localhost:8080/api/v1/disbursements/check-status?referenceId=YOUR_REFERENCE_ID" \
  -H "x-api-key: YOUR_API_KEY"
```

### Wallet Balance

```bash
curl -X GET http://localhost:8080/api/v1/merchants/balance \
  -H "x-api-key: YOUR_API_KEY"
```

Response:

```json
{
  "success": true,
  "message": "Wallet balance retrieved successfully.",
  "data": {
    "balance": 9850.25
  }
}
```

### Health Check

```bash
curl http://localhost:8080/health
```

```json
{"status":"ok"}
```

## Admin Dashboard

Open `http://localhost:8080/admin/` in your browser. No login required.

From the dashboard you can:

- **View wallets** — see all wallets with masked API keys, balances, and status
- **Create wallets** — generate new wallets with auto-generated API keys and till numbers
- **Toggle wallets** — activate/deactivate wallets (deactivated wallets return 403)
- **View transactions** — see the 20 most recent transactions across all wallets
- **Wallet details** — click a wallet name to see its full API key and transaction history
- **Edit simulation config** — tune success rates per payment provider, set processing delays, enable random timeouts
- **Reset database** — wipe everything and start fresh (with confirmation page)

## Simulation Config

Control how the mock behaves via the admin UI at `/admin/config`:

| Setting | Default | Description |
|---------|---------|-------------|
| MTN Success Rate | 80% | Chance of MTN mobile money transactions succeeding |
| Airtel Success Rate | 80% | Chance of Airtel mobile money transactions succeeding |
| Zamtel Success Rate | 80% | Chance of Zamtel mobile money transactions succeeding |
| Card Success Rate | 80% | Chance of card transactions succeeding |
| Bank Success Rate | 80% | Chance of bank transfers succeeding |
| Processing Delay | 3s | Simulated processing time before status changes |
| Enable Random Timeouts | off | Whether to randomly timeout some requests |
| Timeout Probability | 5% | Chance of a random timeout (when enabled) |

Failed transactions include realistic error messages per provider (e.g., MTN balance errors, Airtel user-not-found, bank account mismatches).

## Callbacks

When a `callbackUrl` is provided in a request, the server will POST the transaction result to that URL after processing completes:

```json
{
  "id": "txn-uuid",
  "referenceId": "order-001",
  "identifier": "LIPILA-XXXXXXXX",
  "type": "Collection",
  "paymentType": "MtnMoney",
  "status": "Successful",
  "amount": 100.00,
  "currency": "ZMW",
  "accountNumber": "260971234567",
  "narration": "Payment for order 001",
  "externalId": "MP250101.1234.C5678",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

Callbacks retry up to **3 times** with exponential backoff (2s, 4s, 8s). Each attempt is logged in the database.

## Project Structure

```
mock-lipila/
├── main.go                          # Entry point, server setup, graceful shutdown
├── go.mod
├── go.sum
├── internal/
│   ├── api/
│   │   ├── handlers.go              # API endpoint handlers
│   │   ├── middleware.go            # Auth, CORS, rate limiting, logging
│   │   └── routes.go               # Route registration
│   ├── admin/
│   │   ├── handlers.go              # Admin UI handlers
│   │   └── templates.go            # Embedded HTML templates
│   ├── database/
│   │   ├── db.go                    # SQLite operations, migrations
│   │   └── models.go               # Data models (Wallet, Transaction, etc.)
│   └── simulator/
│       ├── simulator.go            # Simulator core
│       ├── collections.go          # Collection processing logic
│       ├── disbursements.go        # Disbursement processing logic
│       └── callbacks.go            # Callback delivery with retry
└── README.md
```

## Troubleshooting

### "CGO_ENABLED" / "gcc not found" errors during build

SQLite requires CGO (a C compiler). Install one:

- **Windows**: Install [MSYS2](https://www.msys2.org/) or [TDM-GCC](https://jmeubank.github.io/tdm-gcc/), then ensure `gcc` is on your PATH
- **macOS**: Run `xcode-select --install`
- **Linux**: `sudo apt install build-essential` (Debian/Ubuntu) or `sudo dnf install gcc` (Fedora)

### "address already in use" on startup

Another process is using port 8080. Either stop that process or use a different port:

```bash
PORT=9090 ./lipila-mock.exe
```

### Database is locked / corrupt

Delete the database file and restart. The server will create a fresh one:

```bash
rm lipila.db
./lipila-mock.exe
```

### API returns 401 Unauthorized

- Check that you're sending the `x-api-key` header (lowercase)
- Copy the exact API key from the startup log or admin dashboard
- API keys start with `Lsk_`

### API returns 403 Forbidden

The wallet associated with your API key has been deactivated. Re-activate it from the admin dashboard or create a new wallet.

### API returns 429 Too Many Requests

Rate limit is 100 requests per minute per API key. Wait 60 seconds or use a different API key.

### Callbacks not being received

- Ensure your callback URL is reachable from the machine running the mock
- For local development, use a tool like [ngrok](https://ngrok.com) to expose your local server
- Check callback logs in the database or wallet detail page in admin

### Windows: "This app can't run on your PC"

You may have built for the wrong architecture. Rebuild with:

```bash
set GOARCH=amd64
go build -ldflags="-s -w" -o lipila-mock.exe .
```

## License

This project is a development tool for testing against the Lipila payment API. Not affiliated with Lipila.
