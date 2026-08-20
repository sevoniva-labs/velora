package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type Envelope struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	Data        any          `json:"data"`
	Error       string       `json:"error,omitempty"`
	RequestID   string       `json:"request_id,omitempty"`
	TraceID     string       `json:"trace_id,omitempty"`
	Timestamp   string       `json:"timestamp"`
	FieldErrors []FieldError `json:"field_errors,omitempty"`
}
type FieldError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func Success(w http.ResponseWriter, status int, data any, requestID, traceID string) {
	JSON(w, status, Envelope{
		Code: NumericCode("SUCCESS"), Message: "success", Data: data,
		RequestID: requestID, TraceID: traceID, Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func Error(w http.ResponseWriter, status int, symbol, msg, requestID string, traceIDs ...string) {
	traceID := ""
	if len(traceIDs) > 0 {
		traceID = traceIDs[0]
	}
	JSON(w, status, Envelope{
		Code: NumericCode(symbol), Error: symbol, Message: msg, Data: nil,
		RequestID: requestID, TraceID: traceID, Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, max int64, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Reject trailing JSON values.
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return errors.New("request body contains multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
