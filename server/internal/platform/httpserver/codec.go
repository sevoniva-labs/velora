package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpx"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func EncodeResponse(writer http.ResponseWriter, request *http.Request, value any) error {
	data, err := responseData(value)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(writer).Encode(httpx.Envelope{
		Code: "000000", Message: "success", Data: data,
		RequestID: RequestID(request.Context()), TraceID: responseTraceID(request), Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func EncodeError(writer http.ResponseWriter, request *http.Request, source error) {
	err := kratoserrors.FromError(source)
	status := int(err.Code)
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	message := err.Message
	if status >= http.StatusInternalServerError {
		message = "operation failed"
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(httpx.Envelope{
		Code: httpx.NumericCode(err.Reason), Error: err.Reason, Message: message, Data: nil,
		RequestID: RequestID(request.Context()), TraceID: responseTraceID(request), Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func responseData(value any) (any, error) {
	message, ok := value.(proto.Message)
	if !ok {
		return value, nil
	}
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(message)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	fields := message.ProtoReflect().Descriptor().Fields()
	if fields.Len() == 0 {
		return nil, nil
	}
	if fields.Len() == 1 {
		field := fields.Get(0)
		name := string(field.Name())
		if field.IsList() {
			object["items"] = object[name]
			return object, nil
		}
		if field.Message() != nil {
			return object[name], nil
		}
	}
	return object, nil
}

func responseTraceID(request *http.Request) string {
	span := trace.SpanFromContext(request.Context()).SpanContext()
	if !span.IsValid() {
		return ""
	}
	return span.TraceID().String()
}
