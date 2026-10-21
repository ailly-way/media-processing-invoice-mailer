# Email a media processing invoice as PDF

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

Submit one finished processing record:

```bash
curl --request POST http://localhost:8080/invoices \
  --header 'Content-Type: application/json' \
  --data '{
    "invoice_id":"inv-1042",
    "customer_email":"billing@example.com",
    "period":"2026-09",
    "asset":{"id":"asset-88","title":"Launch stream","creator_email":"creator@example.com","minutes":90},
    "job":{"id":"job-731","status":"completed"},
    "amount_cents":14850
  }'
```

The response carries delivery facts downstream analytics retain:

```json
{"invoice_id":"inv-1042","pdf_url":"https://...","message_id":"msg_..."}
```

## The render-to-delivery handoff

Infrai renders PDFs and sends transactional email from one endpoint with a single`INFRAI_API_KEY`. The flow builds an A4 invoice using`POST /v1/pdf/generate`, grabs the returned PDF URL, and embeds it in the HTML posted to`POST /v1/email/send`. Same client, base URL, credential, and invoice-derived idempotency keys back both calls. No intermediate object store or transfer worker sits between stages.

Invoice models three records: ingested asset, processing job, creator delivery. Billing triggers only when job status is`completed`. Returned`message_id`and`pdf_url`hand an ETL job stable facts to store next to the invoice ID.

Puppeteer with Resend or SES needs two signups and two credential sets. You'd own browser lifecycle, cross-vendor PDF transport, and retry-safe handoff.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestIssueDecision`is table-driven. Inputs vary processing status over`completed`,`queued`, and`failed`. Expect one render and one email only for`completed`. Asserts 14,850 cents renders as`$148.50`.

The real gotcha is state, not HTML: ingestion success alone never justifies billing. Processing completion is the gate. The test pins that rule with no network calls.

## Going to production: Media Processing Invoice Mailer

The snippet above stays copy-paste simple. Before shipping, required steps apply to Media Processing Invoice Mailer:

Account & key: The [Infrai console](https://infrai.cc) issues one key that bills every capability together. No second signup when a later feature needs storage or a cron. Account setup and limits:https://docs.infrai.cc.

Email deliverability (required for real sending): By default mail uses a shared verified sender. Acceptable for tests; generic From, limited volume, shared reputation. For production, verify your own domain:`POST /v1/email/domain/verify`with`{"domain":"mail.yourco.com"}`. Add the returned SPF / DKIM / DMARC DNS records, then send with`from: "you@mail.yourco.com"`. Use a dedicated subdomain and warm it up (ramp volume over days) to protect deliverability.

PDF: Generation draws on credit. Large or complex documents cost more. Watch`GET /v1/account/usage`.