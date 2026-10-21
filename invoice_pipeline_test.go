package invoicepipeline

import (
	"context"
	"strings"
	"testing"
)

type recordingMailer struct {
	pdfCalls   int
	emailCalls int
	pdfHTML    string
}

func (m *recordingMailer) GeneratePDF(_ context.Context, body, _ string) (string, error) {
	m.pdfCalls++
	m.pdfHTML = body
	return "https://documents.example/inv-1042.pdf", nil
}

func (m *recordingMailer) SendEmail(_ context.Context, _, _, _, _ string) (string, error) {
	m.emailCalls++
	return "msg_1042", nil
}

func TestIssueDecision(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantErr    bool
		wantCalls  int
		wantAmount string
	}{
		{name: "completed job is rendered and delivered", status: "completed", wantCalls: 1, wantAmount: "$148.50"},
		{name: "queued job is not invoiced", status: "queued", wantErr: true},
		{name: "failed job is not invoiced", status: "failed", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailer := &recordingMailer{}
			pipeline := NewInvoicePipeline(mailer)
			got, err := pipeline.Issue(context.Background(), fixture(tt.status))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Issue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if mailer.pdfCalls != tt.wantCalls || mailer.emailCalls != tt.wantCalls {
				t.Fatalf("calls pdf=%d email=%d, want %d each", mailer.pdfCalls, mailer.emailCalls, tt.wantCalls)
			}
			if !tt.wantErr {
				if got.MessageID != "msg_1042" || !strings.Contains(mailer.pdfHTML, tt.wantAmount) {
					t.Fatalf("delivery=%+v html=%q", got, mailer.pdfHTML)
				}
			}
		})
	}
}

func fixture(status string) InvoiceRequest {
	return InvoiceRequest{
		InvoiceID: "inv-1042", CustomerEmail: "billing@example.com", Period: "2026-09",
		Asset: Asset{ID: "asset-88", Title: "Launch stream", CreatorEmail: "creator@example.com", Minutes: 90},
		Job:   ProcessingJob{ID: "job-731", Status: status}, AmountCents: 14850,
	}
}
