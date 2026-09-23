package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// HandlerFunction defines the handler signature for Lambda
type HandlerFunction func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

// Method defines the supported HTTP methods
type Method string

// Enum values for Method
const (
	GET     Method = "GET"
	POST    Method = "POST"
	PUT     Method = "PUT"
	DELETE  Method = "DELETE"
	PATCH   Method = "PATCH"
	OPTIONS Method = "OPTIONS"
)

// Route represents a single route
type Route struct {
	Method  Method
	Path    string
	Handler HandlerFunction
}

// Server encapsulates both Lambda and local server behavior
type Server struct {
	router         *gin.Engine
	lambda         bool
	allowedMethods map[Method]bool // Tracks allowed methods for CORS
	routes         []Route         // Tracks all mounted routes
}

// NewServer creates a new server instance
func NewServer() *Server {
	server := &Server{
		router:         gin.Default(),
		lambda:         os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "",
		allowedMethods: make(map[Method]bool),
		routes:         []Route{},
	}
	return server
}

// MountEndpoint adds an endpoint with a specified handler
func (s *Server) MountEndpoint(method Method, path string, handler HandlerFunction) {
	// Convert {param} to :param for Gin compatibility
	convertedPath := strings.ReplaceAll(path, "{", ":")
	convertedPath = strings.ReplaceAll(convertedPath, "}", "")

	log.Printf("[Server] Mounting endpoint: %s %s", method, convertedPath)
	s.allowedMethods[method] = true // Track allowed methods dynamically
	s.routes = append(s.routes, Route{Method: method, Path: convertedPath, Handler: handler})
}

// Serve starts the server
func (s *Server) Serve(port string) {
	if s.lambda {
		// Lambda mode: Start Lambda with a single handler
		log.Println("[Server] Running in Lambda mode")
		lambda.Start(s.handleLambdaEvent)
	} else {
		s.handleServerStart(port)
	}
}

// handleLambdaRequest handles all Lambda requests dynamically, extracting path parameters
func (s *Server) handleLambdaEvent(ctx context.Context, payload json.RawMessage) (interface{}, error) {
	var envelope struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}
	if envelope.Version == "2.0" {
		var request events.APIGatewayV2HTTPRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, err
		}
		response, err := s.handleLambdaRequest(ctx, v2ToV1Request(request))
		return v1ToV2Response(response), err
	}

	var request events.APIGatewayProxyRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	return s.handleLambdaRequest(ctx, request)
}

// handleLambdaRequest handles API Gateway REST API requests after event normalization.
func (s *Server) handleLambdaRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("[Lambda] Received request: Method=%s Path=%s", req.HTTPMethod, req.Path)

	// Handle OPTIONS requests for CORS
	if req.HTTPMethod == "OPTIONS" {
		return s.handleOptionsResponse(), nil
	}

	// Match routes dynamically
	for _, route := range s.routes {
		if matchPath(req.Path, route.Path) && strings.EqualFold(req.HTTPMethod, string(route.Method)) {
			log.Printf("[Lambda] Matched route: %s %s", route.Method, route.Path)

			// Extract path parameters from request
			pathParams := extractPathParams(req.Path, route.Path)
			req.PathParameters = pathParams

			return route.Handler(ctx, req)
		}
	}

	// No matching route
	log.Printf("[Lambda] No handler found for: Method=%s Path=%s", req.HTTPMethod, req.Path)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Not Found"}, nil
}

func v2ToV1Request(req events.APIGatewayV2HTTPRequest) events.APIGatewayProxyRequest {
	path := req.RawPath
	if path == "" {
		path = req.RequestContext.HTTP.Path
	}
	return events.APIGatewayProxyRequest{
		HTTPMethod:            req.RequestContext.HTTP.Method,
		Path:                  path,
		Headers:               req.Headers,
		QueryStringParameters: req.QueryStringParameters,
		PathParameters:        req.PathParameters,
		Body:                  req.Body,
		IsBase64Encoded:       req.IsBase64Encoded,
	}
}

func v1ToV2Response(response events.APIGatewayProxyResponse) events.APIGatewayV2HTTPResponse {
	return events.APIGatewayV2HTTPResponse{
		StatusCode:        response.StatusCode,
		Headers:           response.Headers,
		MultiValueHeaders: response.MultiValueHeaders,
		Body:              response.Body,
		IsBase64Encoded:   response.IsBase64Encoded,
	}
}

// matchPath matches a request path against a registered route, handling path parameters
func matchPath(requestPath, routePath string) bool {
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")
	routeParts := strings.Split(strings.Trim(routePath, "/"), "/")

	if len(requestParts) != len(routeParts) {
		return false
	}

	for i := range requestParts {
		if routeParts[i] == "" || routeParts[i] == requestParts[i] || strings.HasPrefix(routeParts[i], ":") {
			continue
		}
		return false
	}
	return true
}

// extractPathParams extracts path parameters from a request based on the route definition
func extractPathParams(requestPath, routePath string) map[string]string {
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")
	routeParts := strings.Split(strings.Trim(routePath, "/"), "/")

	params := make(map[string]string)
	for i := range routeParts {
		if strings.HasPrefix(routeParts[i], ":") {
			paramName := strings.TrimPrefix(routeParts[i], ":")
			params[paramName] = requestParts[i]
		}
	}
	return params
}

// Local server startup logic
func (s *Server) handleServerStart(port string) {
	log.Println("[Server] Running in local server mode")
	s.router.Use(s.setupCORS())

	for _, route := range s.routes {
		s.router.Handle(string(route.Method), route.Path, func(c *gin.Context) {
			ctx := c.Request.Context()
			body, _ := c.GetRawData()
			query := c.Request.URL.Query()

			// Convert query parameters to a map
			queryParams := make(map[string]string)
			for key := range query {
				queryParams[key] = query.Get(key)
			}

			// Convert request headers to a map
			headers := make(map[string]string)
			for key := range c.Request.Header {
				headers[key] = c.Request.Header.Get(key)
			}

			// Extract path parameters from Gin context
			pathParams := make(map[string]string)
			for _, param := range c.Params {
				pathParams[param.Key] = param.Value
			}

			response := serveHTTPHandler(ctx, route.Handler, string(body), queryParams, headers, pathParams, c.Request.Method, c.Request.URL.Path)
			for key, value := range response.Headers {
				c.Header(key, value)
			}
			for key, values := range response.MultiValueHeaders {
				for _, value := range values {
					c.Writer.Header().Add(key, value)
				}
			}
			responseBody := []byte(response.Body)
			if response.IsBase64Encoded {
				decoded, err := base64.StdEncoding.DecodeString(response.Body)
				if err != nil {
					c.String(http.StatusInternalServerError, "Internal Server Error")
					return
				}
				responseBody = decoded
			}
			c.Data(response.StatusCode, response.Headers["Content-Type"], responseBody)
		})
	}

	if port == "" {
		port = "8080"
	}
	log.Printf("[Server] Server running on port %s", port)
	s.router.Run(":" + port)
}

// handleOptionsResponse generates a response for OPTIONS (CORS preflight requests)
func (s *Server) handleOptionsResponse() events.APIGatewayProxyResponse {
	allowedMethods := s.getAllowedMethods()
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": strings.Join(allowedMethods, ", "),
		"Access-Control-Allow-Headers": "Content-Type, Authorization, Accept-Language",
	}
	log.Printf("[Lambda] Responding to OPTIONS with headers: %+v", headers)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    headers,
	}
}

// serveHTTPHandler serves HTTP request handler with path parameters
func serveHTTPHandler(ctx context.Context, handler HandlerFunction, body string, query map[string]string, headers map[string]string, pathParams map[string]string, method, path string) events.APIGatewayProxyResponse {
	req := events.APIGatewayProxyRequest{
		HTTPMethod:            method,
		Path:                  path,
		Body:                  body,
		QueryStringParameters: query,
		Headers:               headers,
		PathParameters:        pathParams,
	}

	response, err := handler(ctx, req)
	if err != nil {
		log.Printf("Handler error: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Internal Server Error"}
	}

	return response
}

// setupCORS dynamically generates the CORS configuration
func (s *Server) setupCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     s.getAllowedMethods(),
		AllowHeaders:     []string{"Content-Type", "Authorization", "Accept-Language"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}

// Router exposes the internal Gin engine for raw access.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// getAllowedMethods returns the dynamically tracked allowed methods
func (s *Server) getAllowedMethods() []string {
	methods := []string{string(OPTIONS)}
	for method := range s.allowedMethods {
		methods = append(methods, string(method))
	}
	slices.Sort(methods)
	return methods
}
