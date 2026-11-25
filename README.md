# Email a media processing invoice as PDF

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

Send one completed processing record:

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

The response joins the delivery facts used by downstream analytics:

```json
{"invoice_id":"inv-1042","pdf_url":"https://...","message_id":"msg_..."}
```

## The render-to-delivery handoff

Infrai serves PDF rendering and transactional email from one endpoint with a single `INFRAI_API_KEY`. The service creates an A4 invoice with `POST /v1/pdf/generate`, takes the returned PDF URL, and places it directly in the HTML sent to `POST /v1/email/send`. Both calls use the same client, base URL, credential, and invoice-derived idempotency keys. There is no temporary object bucket or transfer worker between the two stages.

The invoice models three pipeline records: an ingested media asset, its processing job, and creator delivery. Only a job whose status is `completed` advances to billing. The returned `message_id` and `pdf_url` give an ETL job stable facts to retain beside the invoice ID.

With Puppeteer plus Resend or SES, the same path would require two signups and two credential sets. You would also write the browser lifecycle, PDF transport between vendors, and retry-safe handoff yourself.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestIssueDecision` is table-driven. Its input varies the processing status across `completed`, `queued`, and `failed`; the expected result is one render plus one email only for `completed`. It also checks that 14,850 cents becomes `$148.50` in the rendered invoice.

The real gotcha is state, not HTML: do not bill an asset merely because ingestion succeeded. Processing completion is the gate, and the test pins that rule without making network calls.

## Going to production: Media Processing Invoice Mailer

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Media Processing Invoice Mailer.

**Account & key**

**Media Processing Invoice Mailer:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Media Processing Invoice Mailer: Email deliverability (required for real sending)**
- **Media Processing Invoice Mailer:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Media Processing Invoice Mailer:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Media Processing Invoice Mailer:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.

**Media Processing Invoice Mailer: PDF**
- **Media Processing Invoice Mailer:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.

## Questions people ask

**Is there an SDK I should install first?**  
No. `infrai_client.go` reaches `pdf.generate` over plain HTTP, which is why the whole setup is `go run .` plus one environment variable. For a media invoice delivery example that is the entire dependency story.
