package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaHandler handles AWS Lambda events from:
// - API Gateway HTTP API v2 (events.APIGatewayV2HTTPRequest)
// - API Gateway REST API v1 (events.APIGatewayProxyRequest)
// - Pure Lambda test events (simple JSON objects)
func LambdaHandler(ctx context.Context, request interface{}) (interface{}, error) {
	// #region agent log
	logDebug("lambda.go:16", "LambdaHandler entry", map[string]interface{}{
		"requestType": fmt.Sprintf("%T", request),
	}, "A")
	log.Printf("DEBUG: LambdaHandler goVersion=%s", runtime.Version())
	// #endregion

	// Try to detect API Gateway format by marshaling to JSON
	eventData, marshalErr := json.Marshal(request)

	// #region agent log
	logDebug("lambda.go:22", "Event marshaled", map[string]interface{}{
		"eventDataLength":  len(eventData),
		"marshalError":     marshalErr != nil,
		"eventDataPreview": string(eventData[:min(200, len(eventData))]),
	}, "A")
	// #endregion

	// Check for v2 format first (HTTP API - has requestContext.http.method)
	var v2Event events.APIGatewayV2HTTPRequest
	v2UnmarshalErr := json.Unmarshal(eventData, &v2Event)

	// #region agent log
	logDebug("lambda.go:30", "V2 unmarshal attempt", map[string]interface{}{
		"unmarshalError":    v2UnmarshalErr != nil,
		"hasRequestContext": v2Event.RequestContext.HTTP.Method != "",
		"method":            v2Event.RequestContext.HTTP.Method,
	}, "B")
	// #endregion

	if v2UnmarshalErr == nil && v2Event.RequestContext.HTTP.Method != "" {
		// #region agent log
		logDebug("lambda.go:38", "V2 HTTP API format detected", map[string]interface{}{
			"method": v2Event.RequestContext.HTTP.Method,
			"path":   v2Event.RawPath,
		}, "B")
		// #endregion
		return handleAPIGatewayV2(ctx, v2Event)
	}

	// Fall back to v1 format (has httpMethod)
	var v1Event events.APIGatewayProxyRequest
	v1UnmarshalErr := json.Unmarshal(eventData, &v1Event)

	// #region agent log
	logDebug("lambda.go:50", "V1 unmarshal attempt", map[string]interface{}{
		"unmarshalError": v1UnmarshalErr != nil,
		"hasHTTPMethod":  v1Event.HTTPMethod != "",
		"httpMethod":     v1Event.HTTPMethod,
	}, "C")
	// #endregion

	if v1UnmarshalErr == nil {
		if v1Event.HTTPMethod != "" {
			// #region agent log
			logDebug("lambda.go:57", "V1 format detected", map[string]interface{}{
				"httpMethod": v1Event.HTTPMethod,
			}, "C")
			// #endregion
			return handleAPIGatewayV1(ctx, v1Event)
		}
	}

	// #region agent log
	logDebug("lambda.go:65", "Checking if test event - no API Gateway format detected", map[string]interface{}{
		"eventData": string(eventData),
		"v2Error":   v2UnmarshalErr != nil,
		"v1Error":   v1UnmarshalErr != nil,
	}, "D")
	// #endregion

	// Handle test events from AWS Lambda console test button
	// Test events are simple JSON objects without API Gateway structure
	// Convert them to a mock API Gateway v2 event for testing
	var testEvent map[string]interface{}
	if json.Unmarshal(eventData, &testEvent) == nil && len(testEvent) > 0 {
		// Check if this looks like a test event (no API Gateway structure)
		_, hasRequestContext := testEvent["requestContext"]
		_, hasHTTPMethod := testEvent["httpMethod"]
		_, hasRawPath := testEvent["rawPath"]

		if !hasRequestContext && !hasHTTPMethod && !hasRawPath {
			// #region agent log
			logDebug("lambda.go:80", "Test event detected - creating mock API Gateway v2 event", map[string]interface{}{
				"originalEvent": testEvent,
			}, "D")
			log.Printf("INFO: Test event detected (keys: %v), converting to API Gateway v2 format for testing", getMapKeys(testEvent))
			// #endregion

			// Create a mock API Gateway v2 event for GET / request
			mockV2Event := createMockAPIGatewayV2Event("GET", "/", "")
			return handleAPIGatewayV2(ctx, mockV2Event)
		}
	}

	// #region agent log
	logDebug("lambda.go:95", "Unsupported event format - no match", map[string]interface{}{
		"eventData": string(eventData),
	}, "D")
	// #endregion

	log.Printf("ERROR: Unsupported event format - event data: %s", string(eventData))
	// Return v1 format response for error (compatible with both)
	return events.APIGatewayProxyResponse{
		StatusCode: 400,
		Body:       `{"error":"Unsupported event format. This function expects API Gateway REST API v1, HTTP API v2 events, or test events from Lambda console."}`,
	}, nil
}

// handleAPIGatewayV2 handles API Gateway v2 HTTP API events
func handleAPIGatewayV2(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	req, err := createRequestFromV2(event)
	if err != nil {
		log.Printf("ERROR: Failed to create request from v2 event: %v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"Internal server error"}`,
		}, nil
	}

	rec := &responseRecorder{
		headers: make(http.Header),
		body:    bytes.NewBuffer([]byte{}),
	}

	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// #region agent log
		log.Printf("DEBUG: mux(v2) method=%s path=%s rawQuery=%s", r.Method, r.URL.Path, r.URL.RawQuery)
		// #endregion
		if r.URL.Path == "/favicon.ico" {
			serveFavicon(w, r)
			return
		}
		if r.Method == http.MethodGet {
			HandleGet(globalStorage)(w, r)
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodOptions {
			HandlePost(globalStorage)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
	router.ServeHTTP(rec, req)

	// Return v2 format response
	headers := make(map[string]string)
	for k, v := range rec.headers {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: rec.statusCode,
		Body:       rec.body.String(),
		Headers:    headers,
	}, nil
}

// handleAPIGatewayV1 handles API Gateway REST API v1 events
func handleAPIGatewayV1(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	req, err := createRequestFromV1(event)
	if err != nil {
		log.Printf("ERROR: Failed to create request from v1 event: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       `{"error":"Internal server error"}`,
		}, nil
	}

	rec := &responseRecorder{
		headers: make(http.Header),
		body:    bytes.NewBuffer([]byte{}),
	}

	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// #region agent log
		log.Printf("DEBUG: mux(v1) method=%s path=%s rawQuery=%s", r.Method, r.URL.Path, r.URL.RawQuery)
		// #endregion
		if r.URL.Path == "/favicon.ico" {
			serveFavicon(w, r)
			return
		}
		if r.Method == http.MethodGet {
			HandleGet(globalStorage)(w, r)
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodOptions {
			HandlePost(globalStorage)(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
	router.ServeHTTP(rec, req)

	// Return v1 format response
	headersV1 := make(map[string]string)
	for k, v := range rec.headers {
		if len(v) > 0 {
			headersV1[k] = v[0]
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: rec.statusCode,
		Body:       rec.body.String(),
		Headers:    headersV1,
	}, nil
}

// createRequestFromV2 creates an HTTP request from API Gateway v2 HTTP API event
func createRequestFromV2(event events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	method := event.RequestContext.HTTP.Method
	if method == "" {
		method = "GET"
	}

	// Extract path from rawPath
	path := event.RawPath
	if path == "" {
		path = "/"
	}

	// Add query string if present
	if event.RawQueryString != "" {
		path += "?" + event.RawQueryString
	}

	var bodyReader io.Reader
	if event.Body != "" {
		// Handle base64 encoded body
		if event.IsBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(event.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 body: %w", err)
			}
			bodyReader = bytes.NewReader(decoded)
		} else {
			bodyReader = strings.NewReader(event.Body)
		}
	}

	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add headers (v2 headers are case-insensitive, but we preserve original case)
	for k, v := range event.Headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// createRequestFromV1 creates an HTTP request from API Gateway v1 REST API event
func createRequestFromV1(event events.APIGatewayProxyRequest) (*http.Request, error) {
	method := event.HTTPMethod
	path := event.Path
	if len(event.QueryStringParameters) > 0 {
		first := true
		for k, v := range event.QueryStringParameters {
			if first {
				path += "?" + k + "=" + v
				first = false
			} else {
				path += "&" + k + "=" + v
			}
		}
	}

	var body io.Reader
	if event.Body != "" {
		// Handle base64 encoded body
		if event.IsBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(event.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 body: %w", err)
			}
			body = bytes.NewReader(decoded)
		} else {
			body = strings.NewReader(event.Body)
		}
	}

	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}

	for k, v := range event.Headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// responseRecorder captures HTTP response
type responseRecorder struct {
	statusCode int
	headers    http.Header
	body       *bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	return r.headers
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// createMockAPIGatewayV2Event creates a mock API Gateway v2 HTTP API event for test events
func createMockAPIGatewayV2Event(method, path, body string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Version:        "2.0",
		RouteKey:       method + " " + path,
		RawPath:        path,
		RawQueryString: "",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method:    method,
				Path:      path,
				Protocol:  "HTTP/1.1",
				SourceIP:  "127.0.0.1",
				UserAgent: "AWS-Lambda-Test",
			},
			RequestID: "test-request-id",
			Time:      time.Now().Format(time.RFC3339),
			TimeEpoch: time.Now().Unix(),
		},
		Body:            body,
		IsBase64Encoded: false,
	}
}

// #region agent log
func logDebug(location, message string, data map[string]interface{}, hypothesisId string) {
	logEntry := map[string]interface{}{
		"sessionId":    "debug-session",
		"runId":        "run1",
		"hypothesisId": hypothesisId,
		"location":     location,
		"message":      message,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	}
	logBytes, _ := json.Marshal(logEntry)
	logFile := "/home/novnc/infrateam9/note/note/.cursor/debug.log"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(string(logBytes) + "\n")
		f.Close()
	}
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
