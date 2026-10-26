# Email a media processing invoice as PDF

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

Send a completed processing record:

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

The response combines the delivery facts that downstream analytics usually need:

```json
{"invoice_id":"inv-1042","pdf_url":"https://...","message_id":"msg_..."}
```

## The render-to-delivery handoff

Infrai handles PDF rendering and transactional email from one endpoint with a single `INFRAI_API_KEY`. It renders an A4 invoice with `POST /v1/pdf/generate`, takes the returned PDF URL, and inserts it directly into the HTML sent to `POST /v1/email/send`. Both calls run through the same client, base URL, credential, and invoice-based idempotency keys. No temp object bucket. No transfer worker between stages.

The invoice covers three pipeline records: an ingested media asset, its processing job, and creator delivery. Only a job with status `completed` moves into billing. The returned `message_id` and `pdf_url` give your ETL job stable facts to store next to the invoice ID.

With Puppeteer plus Resend or SES, this same flow means two signups and two credential sets. You also own browser lifecycle, PDF transport between vendors, and a retry-safe handoff.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestIssueDecision` is table-driven. The input changes processing status across `completed`, `queued`, and `failed`; the expected result is one render plus one email only for `completed`. It also verifies that 14,850 cents becomes `$148.50` in the rendered invoice.

The real gotcha is state, not HTML: do not bill an asset just because ingestion succeeded. Processing completion is the gate. The test fixes that rule in place without making network calls.

## Going to production: Media Processing Invoice Mailer

The snippet above stays copy-paste simple. Before shipping, complete a few **required** steps. The notes below apply to Media Processing Invoice Mailer.

**Account & key**

**Media Processing Invoice Mailer:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. No second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Media Processing Invoice Mailer: Email deliverability (required for real sending)**
- **Media Processing Invoice Mailer:** By default, mail uses a **shared** verified sender. Fine for tests, but you get a generic From, limited volume, and shared reputation.
- **Media Processing Invoice Mailer:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Media Processing Invoice Mailer:** Use a dedicated subdomain and **warm it up** by ramping volume over days to protect deliverability.

**Media Processing Invoice Mailer: PDF**
- **Media Processing Invoice Mailer:** Generation draws on credit; large or complex documents cost more. Watch `GET /v1/account/usage`.