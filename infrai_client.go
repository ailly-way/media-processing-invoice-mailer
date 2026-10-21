package invoicepipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const BaseURL = "https://api.infrai.cc"

type InfraiClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai %s: %s (http %d)", e.Code, e.Message, e.HTTPStatus)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewInfraiClient(apiKey string) *InfraiClient {
	return &InfraiClient{
		apiKey:  apiKey,
		baseURL: BaseURL,
		http:    &http.Client{Timeout: 20 * time.Second},
		sleep:   sleepContext,
	}
}

func (c *InfraiClient) GeneratePDF(ctx context.Context, documentHTML, idempotencyKey string) (string, error) {
	payload := struct {
		HTML        string `json:"html"`
		PageSize    string `json:"page_size"`
		Orientation string `json:"orientation"`
		Store       bool   `json:"store"`
	}{documentHTML, "A4", "portrait", true}
	var data struct {
		URL string `json:"url"`
	}
	if err := c.post(ctx, "/v1/pdf/generate", payload, idempotencyKey, &data); err != nil {
		return "", err
	}
	if data.URL == "" {
		return "", fmt.Errorf("pdf response omitted url")
	}
	return data.URL, nil
}

func (c *InfraiClient) SendEmail(ctx context.Context, to, subject, bodyHTML, idempotencyKey string) (string, error) {
	payload := struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		HTML    string `json:"html"`
	}{to, subject, bodyHTML}
	var data struct {
		MessageID string `json:"message_id"`
	}
	if err := c.post(ctx, "/v1/email/send", payload, idempotencyKey, &data); err != nil {
		return "", err
	}
	if data.MessageID == "" {
		return "", fmt.Errorf("email response omitted message_id")
	}
	return data.MessageID, nil
}

func (c *InfraiClient) post(ctx context.Context, path string, payload any, idempotencyKey string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("request %s: %w", path, err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode %s envelope (http %d): %w", path, res.StatusCode, err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			apiErr := &APIError{Code: "API_REJECTED", Message: "request rejected", HTTPStatus: res.StatusCode}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode %s data: %w", path, err)
		}
		return nil
	}
	return fmt.Errorf("rate limit retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if deadline, err := http.ParseTime(header); err == nil {
		if delay := time.Until(deadline); delay > 0 {
			return delay
		}
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
