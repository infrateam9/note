package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaHandler handles AWS Lambda events from API Gateway v1 (REST) or v2 (HTTP)
func LambdaHandler(ctx context.Context, request interface{}) (interface{}, error) {
	eventData, _ := json.Marshal(request)

	// Detect v2 format (HTTP API)
	var v2Event events.APIGatewayV2HTTPRequest
	if json.Unmarshal(eventData, &v2Event) == nil && v2Event.RequestContext.HTTP.Method != "" {
		log.Printf("[DEBUG] Lambda v2: %s %s (IP: %s)",
			v2Event.RequestContext.HTTP.Method,
			v2Event.RawPath,
			v2Event.RequestContext.HTTP.SourceIP,
		)
		return handleAPIGatewayV2(ctx, v2Event)
	}

	// Detect v1 format (REST API)
	var v1Event events.APIGatewayProxyRequest
	if json.Unmarshal(eventData, &v1Event) == nil && v1Event.HTTPMethod != "" {
		log.Printf("[DEBUG] Lambda v1: %s %s (IP: %s)",
			v1Event.HTTPMethod,
			v1Event.Path,
			v1Event.RequestContext.Identity.SourceIP,
		)
		return handleAPIGatewayV1(ctx, v1Event)
	}

	log.Printf("[ERROR] Unsupported event format: %s", string(eventData))
	return events.APIGatewayProxyResponse{
		StatusCode: 400,
		Body:       `{"error":"Unsupported event format"}`,
	}, nil
}

func handleAPIGatewayV2(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	req, _ := createRequestFromV2(event)
	rec := &responseRecorder{
		headers: make(http.Header),
		body:    bytes.NewBuffer([]byte{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			HandleGet(globalStorage)(w, r)
		} else {
			HandlePost(globalStorage)(w, r)
		}
	})
	mux.ServeHTTP(rec, req)

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

func handleAPIGatewayV1(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	req, _ := createRequestFromV1(event)
	rec := &responseRecorder{
		headers: make(http.Header),
		body:    bytes.NewBuffer([]byte{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			HandleGet(globalStorage)(w, r)
		} else {
			HandlePost(globalStorage)(w, r)
		}
	})
	mux.ServeHTTP(rec, req)

	headers := make(map[string]string)
	for k, v := range rec.headers {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: rec.statusCode,
		Body:       rec.body.String(),
		Headers:    headers,
	}, nil
}

func createRequestFromV2(event events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	method := event.RequestContext.HTTP.Method
	path := event.RawPath
	if event.RawQueryString != "" {
		path += "?" + event.RawQueryString
	}

	var body io.Reader = strings.NewReader(event.Body)
	if event.IsBase64Encoded {
		d, _ := base64.StdEncoding.DecodeString(event.Body)
		body = bytes.NewReader(d)
	}

	req, _ := http.NewRequest(method, path, body)
	for k, v := range event.Headers {
		req.Header.Set(k, v)
	}

	// Set RemoteAddr so ClientIP() can fall back to it
	req.RemoteAddr = event.RequestContext.HTTP.SourceIP

	return req, nil
}

func createRequestFromV1(event events.APIGatewayProxyRequest) (*http.Request, error) {
	path := event.Path
	if len(event.QueryStringParameters) > 0 {
		var params []string
		for k, v := range event.QueryStringParameters {
			params = append(params, k+"="+v)
		}
		path += "?" + strings.Join(params, "&")
	}

	var body io.Reader = strings.NewReader(event.Body)
	if event.IsBase64Encoded {
		d, _ := base64.StdEncoding.DecodeString(event.Body)
		body = bytes.NewReader(d)
	}

	req, _ := http.NewRequest(event.HTTPMethod, path, body)
	for k, v := range event.Headers {
		req.Header.Set(k, v)
	}

	// Set RemoteAddr so ClientIP() can fall back to it
	req.RemoteAddr = event.RequestContext.Identity.SourceIP

	return req, nil
}

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
