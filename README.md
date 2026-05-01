# SMS Ingest Lambda

This project is a Go AWS Lambda that receives bank transaction SMS payloads over HTTP, parses them, and appends the parsed transaction to Google Sheets.

## Current Behavior

1. Accepts `POST` requests only
2. Validates `x-auth-token` against the `AUTH_TOKEN` environment variable
3. Parses the JSON request body
4. Resolves the bank config from the SMS sender
5. Tries that bank's configured message patterns until one matches
6. Appends the parsed transaction to the configured Google Sheet

## Request Body

Expected JSON:

```json
{
  "sender": "NationsSMS",
  "message": "A TRANSACTION of LKR 1,185.94 was approved on your A/C No. 200680****580 at UBER EATS CBH. Current Bal LKR 95200.23",
  "received_at": "2026-02-04T12:34:56Z",
  "device_id": "pixel-8"
}
```

`received_at` must be RFC3339.

## Config

`config.yml` uses this shape:

```yml
spreadsheet:
  url: ""
  sheet_name: Transactions

banks:
  - name: "NTB"
    senders:
      - "^NationsSMS$"
    patterns:
      - name: "card_purchase"
        direction: "debit"
        pattern: "^A TRANSACTION of LKR ...$"
      - name: "islips"
        direction: "credit"
        pattern: "^ISLIPS ...$"
```

Each bank entry contains:

- `name`: bank name written to the sheet
- `senders`: one or more sender regex patterns
- `patterns`: one or more message patterns for that bank

Each message pattern contains:

- `name`: pattern name for readability
- `direction`: `credit` or `debit`
- `pattern`: regex used to parse that SMS format

## Environment Variables

- `AUTH_TOKEN`: shared secret expected in the `x-auth-token` header
- `GOOGLE_CREDENTIALS_JSON`: Google service account JSON
- `SPREADSHEET_URL`: optional override for `config.yml` spreadsheet URL

## Output Columns

Rows are appended to Google Sheets in this order:

1. `received_at`
2. `amount`
3. `direction`
4. `description`
5. `account_mask`
6. `sender`
7. `device_id`
8. `bank_name`

## Local Commands

```sh
make test
make vet
make fmt
make build
make package
```

`make build` creates `dist/lambda/bootstrap`.

`make package` creates `dist/lambda/function.zip`.

## Local HTTP Testing

Run the app as a normal local HTTP server:

```sh
LOCAL_HTTP=1 go run .
```

Default local URL:

```text
http://localhost:8080/
```

You can override the port with `PORT`.

Example:

```sh
curl -X POST http://localhost:8080/ \
  -H 'Content-Type: application/json' \
  -H 'x-auth-token: your-auth-token' \
  -d '{
    "sender":"NationsSMS",
    "message":"A TRANSACTION of LKR 1,185.94 was approved on your A/C No. 200680****580 at UBER EATS CBH. Current Bal LKR 95200.23",
    "received_at":"2026-02-04T12:34:56Z",
    "device_id":"pixel-8"
  }'
```
