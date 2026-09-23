package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

type contextKey string

func TestHandleLambdaRequestMatchesRouteAndPropagatesContext(t *testing.T) {
	server := NewServer()
	key := contextKey("request")
	server.MountEndpoint(GET, "/items/{id}", func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		if ctx.Value(key) != "present" {
			t.Fatal("invocation context was not propagated")
		}
		if req.PathParameters["id"] != "42" {
			t.Fatalf("expected path parameter 42, got %#v", req.PathParameters)
		}
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: "ok"}, nil
	})

	ctx := context.WithValue(context.Background(), key, "present")
	response, err := server.handleLambdaRequest(ctx, events.APIGatewayProxyRequest{HTTPMethod: "GET", Path: "/items/42"})
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: %#v, %v", response, err)
	}
}

func TestHandleLambdaEventSupportsHTTPAPIV2(t *testing.T) {
	server := NewServer()
	server.MountEndpoint(POST, "/items/{id}", func(_ context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		if req.HTTPMethod != "POST" || req.Path != "/items/7" || req.PathParameters["id"] != "7" {
			t.Fatalf("unexpected normalized request: %#v", req)
		}
		return events.APIGatewayProxyResponse{StatusCode: http.StatusCreated, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"created":true}`}, nil
	})

	payload, err := json.Marshal(events.APIGatewayV2HTTPRequest{
		Version: "2.0",
		RawPath: "/items/7",
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "POST", Path: "/items/7"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.handleLambdaEvent(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	response, ok := result.(events.APIGatewayV2HTTPResponse)
	if !ok || response.StatusCode != http.StatusCreated || response.Headers["Content-Type"] != "application/json" {
		t.Fatalf("unexpected v2 response: %#v", result)
	}
}

func TestHandleLambdaEventPreservesRESTAPIResponse(t *testing.T) {
	server := NewServer()
	server.MountEndpoint(GET, "/health", func(_ context.Context, _ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent}, nil
	})
	payload, _ := json.Marshal(events.APIGatewayProxyRequest{HTTPMethod: "GET", Path: "/health"})
	result, err := server.handleLambdaEvent(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	response, ok := result.(events.APIGatewayProxyResponse)
	if !ok || response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected REST API response: %#v", result)
	}
}

func TestMatchPath(t *testing.T) {
	for _, test := range []struct {
		request string
		route   string
		want    bool
	}{
		{"/items/42", "/items/:id", true},
		{"/items", "/items/:id", false},
		{"/other/42", "/items/:id", false},
		{"/", "/", true},
	} {
		if got := matchPath(test.request, test.route); got != test.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", test.request, test.route, got, test.want)
		}
	}
}

func TestOptionsIncludeMountedMethods(t *testing.T) {
	server := NewServer()
	server.MountEndpoint(GET, "/health", nil)
	response := server.handleOptionsResponse()
	if response.Headers["Access-Control-Allow-Methods"] != "GET, OPTIONS" {
		t.Fatalf("unexpected methods: %q", response.Headers["Access-Control-Allow-Methods"])
	}
}
