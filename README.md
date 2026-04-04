# SMS Ingest Lambda

This project is a Go AWS Lambda that receives bank transaction SMS payloads over HTTP, parses them, and appends the parsed transaction to Google Sheets.

## Current Behavior

1. Accepts `POST` requests only
2. Validates `x-auth-token` against the `AUTH_TOKEN` environment variable
3. Parses the JSON request body
4. Resolves the bank config from the SMS sender
5. Parses the SMS message using that bank's regex pattern
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
  - name: "nations"
    senders:
      - "^NationsSMS$"
    pattern: "^A TRANSACTION of ...$"

timezone: "UTC"
```

Each bank entry contains:

- `name`: bank name written to the sheet
- `senders`: one or more sender regex patterns
- `pattern`: regex used to parse that bank's SMS format

## Environment Variables

- `AUTH_TOKEN`: shared secret expected in the `x-auth-token` header
- `GOOGLE_CREDENTIALS_JSON`: Google service account JSON
- `SPREADSHEET_URL`: optional override for `config.yml` spreadsheet URL

## Output Columns

Rows are appended to Google Sheets in this order:

1. `received_at`
2. `amount`
3. `currency`
4. `merchant`
5. `balance`
6. `account_mask`
7. `raw_message`
8. `sender`
9. `balance_currency`
10. `bank_name`
11. `device_id`

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
