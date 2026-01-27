package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaHandler handles AWS Lambda events from API Gateway (v1 and v2)
func LambdaHandler(ctx context.Context, request interface{}) (interface{}, error) {
	// Try to detect API Gateway format by marshaling to JSON
	eventData, _ := json.Marshal(request)

	// Check for v2 format first (has requestContext.http.method)
	var v2Event map[string]interface{}
	if err := json.Unmarshal(eventData, &v2Event); err == nil {
		if rc, ok := v2Event["requestContext"].(map[string]interface{}); ok {
			if http, ok := rc["http"].(map[string]interface{}); ok {
				if method, ok := http["method"].(string); ok && method != "" {
					return handleAPIGatewayV2(ctx, v2Event)
				}
			}
		}
	}

	// Fall back to v1 format (has httpMethod)
	var v1Event events.APIGatewayProxyRequest
	if err := json.Unmarshal(eventData, &v1Event); err == nil {
		if v1Event.HTTPMethod != "" {
			return handleAPIGatewayV1(ctx, v1Event)
		}
	}

	log.Println("ERROR: Unsupported event format")
	return events.APIGatewayProxyResponse{
		StatusCode: 400,
		Body:       `{"error":"Unsupported event format"}`,
	}, nil
}

// handleAPIGatewayV2 handles API Gateway v2 events
func handleAPIGatewayV2(ctx context.Context, eventData map[string]interface{}) (map[string]interface{}, error) {
	req, err := createRequestFromV2Generic(eventData)
	if err != nil {
		log.Printf("ERROR: Failed to create request from v2 event: %v", err)
		return map[string]interface{}{
			"statusCode": 500,
			"body":       `{"error":"Internal server error"}`,
		}, nil
	}

	rec := &responseRecorder{
		headers: make(http.Header),
		body:    bytes.NewBuffer([]byte{}),
	}

	router := http.NewServeMux()
	router.HandleFunc("GET /", HandleGet(globalStorage))
	router.HandleFunc("POST /", HandlePost(globalStorage))
	router.ServeHTTP(rec, req)

	// Return v2 format response
	headers := make(map[string]string)
	for k, v := range rec.headers {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return map[string]interface{}{
		"statusCode": rec.statusCode,
		"body":       rec.body.String(),
		"headers":    headers,
	}, nil
}

// handleAPIGatewayV1 handles API Gateway v1 events
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
	router.HandleFunc("GET /", HandleGet(globalStorage))
	router.HandleFunc("POST /", HandlePost(globalStorage))
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

// createRequestFromV2Generic creates an HTTP request from API Gateway v2 event (generic map format)
func createRequestFromV2Generic(eventData map[string]interface{}) (*http.Request, error) {
	// Extract method from requestContext.http.method
	method := "GET"
	if rc, ok := eventData["requestContext"].(map[string]interface{}); ok {
		if httpCtx, ok := rc["http"].(map[string]interface{}); ok {
			if m, ok := httpCtx["method"].(string); ok {
				method = m
			}
		}
	}

	// Extract path from rawPath
	path := "/"
	if rp, ok := eventData["rawPath"].(string); ok {
		path = rp
	}

	// Add query string if present
	if rqs, ok := eventData["rawQueryString"].(string); ok && rqs != "" {
		path += "?" + rqs
	}

	var bodyReader io.Reader
	if body, ok := eventData["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add headers
	if headers, ok := eventData["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vStr, ok := v.(string); ok {
				req.Header.Set(k, vStr)
			}
		}
	}

	return req, nil
}

// createRequestFromV1 creates an HTTP request from API Gateway v1 event
func createRequestFromV1(event events.APIGatewayProxyRequest) (*http.Request, error) {
	method := event.HTTPMethod
	path := event.Path
	if event.QueryStringParameters != nil && len(event.QueryStringParameters) > 0 {
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
		body = strings.NewReader(event.Body)
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
