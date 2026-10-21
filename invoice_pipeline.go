package invoicepipeline

import (
	"context"
	"fmt"
	"html"
	"strings"
)

type Asset struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	CreatorEmail string `json:"creator_email"`
	Minutes      int    `json:"minutes"`
}

type ProcessingJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type InvoiceRequest struct {
	InvoiceID     string        `json:"invoice_id"`
	CustomerEmail string        `json:"customer_email"`
	Period        string        `json:"period"`
	Asset         Asset         `json:"asset"`
	Job           ProcessingJob `json:"job"`
	AmountCents   int           `json:"amount_cents"`
}

type Delivery struct {
	InvoiceID string `json:"invoice_id"`
	PDFURL    string `json:"pdf_url"`
	MessageID string `json:"message_id"`
}

type DocumentMailer interface {
	GeneratePDF(context.Context, string, string) (string, error)
	SendEmail(context.Context, string, string, string, string) (string, error)
}

type InvoicePipeline struct {
	client DocumentMailer
}

func NewInvoicePipeline(client DocumentMailer) *InvoicePipeline {
	return &InvoicePipeline{client: client}
}

func (p *InvoicePipeline) Issue(ctx context.Context, in InvoiceRequest) (Delivery, error) {
	if err := validate(in); err != nil {
		return Delivery{}, err
	}
	if in.Job.Status != "completed" {
		return Delivery{}, fmt.Errorf("processing job %s is %s; invoice requires completed", in.Job.ID, in.Job.Status)
	}

	pdfURL, err := p.client.GeneratePDF(ctx, invoiceHTML(in), "invoice-pdf:"+in.InvoiceID)
	if err != nil {
		return Delivery{}, fmt.Errorf("render invoice: %w", err)
	}

	subject := "Media processing invoice " + in.InvoiceID
	body := fmt.Sprintf("<p>Your media processing invoice is ready.</p><p><a href=\"%s\">Download invoice PDF</a></p>", html.EscapeString(pdfURL))
	messageID, err := p.client.SendEmail(ctx, in.CustomerEmail, subject, body, "invoice-email:"+in.InvoiceID)
	if err != nil {
		return Delivery{}, fmt.Errorf("deliver invoice: %w", err)
	}

	return Delivery{InvoiceID: in.InvoiceID, PDFURL: pdfURL, MessageID: messageID}, nil
}

func validate(in InvoiceRequest) error {
	if strings.TrimSpace(in.InvoiceID) == "" || strings.TrimSpace(in.CustomerEmail) == "" {
		return fmt.Errorf("invoice_id and customer_email are required")
	}
	if strings.TrimSpace(in.Asset.ID) == "" || strings.TrimSpace(in.Job.ID) == "" {
		return fmt.Errorf("asset.id and job.id are required")
	}
	if in.AmountCents < 0 {
		return fmt.Errorf("amount_cents must be non-negative")
	}
	return nil
}

func invoiceHTML(in InvoiceRequest) string {
	return fmt.Sprintf(`<!doctype html><html><body><h1>Invoice %s</h1><dl><dt>Period</dt><dd>%s</dd><dt>Asset</dt><dd>%s (%s)</dd><dt>Processing job</dt><dd>%s</dd><dt>Processed minutes</dt><dd>%d</dd><dt>Creator delivery</dt><dd>%s</dd><dt>Total</dt><dd>%s</dd></dl></body></html>`,
		html.EscapeString(in.InvoiceID), html.EscapeString(in.Period), html.EscapeString(in.Asset.Title), html.EscapeString(in.Asset.ID), html.EscapeString(in.Job.ID), in.Asset.Minutes, html.EscapeString(in.Asset.CreatorEmail), formatMoney(in.AmountCents))
}

func formatMoney(cents int) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}
