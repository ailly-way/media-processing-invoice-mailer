package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	invoicepipeline "example.com/media-invoice-pipeline"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	pipeline := invoicepipeline.NewInvoicePipeline(invoicepipeline.NewInfraiClient(key))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /invoices", func(w http.ResponseWriter, r *http.Request) {
		var input invoicepipeline.InvoiceRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := pipeline.Issue(r.Context(), input)
		if err != nil {
			status := http.StatusUnprocessableEntity
			var apiErr *invoicepipeline.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				status = apiErr.HTTPStatus
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("invoice service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
