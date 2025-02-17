package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"
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
	log.Printf("[Server] Mounting endpoint: %s %s", method, path)
	s.allowedMethods[method] = true // Track allowed methods dynamically
	s.routes = append(s.routes, Route{Method: method, Path: path, Handler: handler})
}

// Serve starts the server
func (s *Server) Serve(port string) {
	if s.lambda {
		// Lambda mode: Start Lambda with a single handler
		log.Println("[Server] Running in Lambda mode")
		lambda.Start(s.handleLambdaRequest)
	} else {
		s.handleServerStart(port)
	}
}

// handleLambdaRequest handles all Lambda requests dynamically
func (s *Server) handleLambdaRequest(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("[Lambda] Received request: Method=%s Path=%s", req.HTTPMethod, req.Path)
	ctx := context.Background()

	// Handle OPTIONS requests for CORS
	if req.HTTPMethod == "OPTIONS" {
		return s.optionsResponse(), nil
	}

	// Match routes dynamically
	for _, route := range s.routes {
		if strings.EqualFold(req.Path, route.Path) && strings.EqualFold(req.HTTPMethod, string(route.Method)) {
			log.Printf("[Lambda] Handling request for: %s %s", route.Method, route.Path)
			return route.Handler(ctx, req)
		}
	}

	// No matching route
	log.Printf("[Lambda] No handler found for: Method=%s Path=%s", req.HTTPMethod, req.Path)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "Not Found"}, nil
}

func (s *Server) handleServerStart(port string) {
	log.Println("[Server] Running in local server mode")
	s.router.Use(s.setupCORS())

	for _, route := range s.routes {
		// Create a closure for each route
		s.router.Handle(string(route.Method), route.Path, func(c *gin.Context) {
			ctx := context.Background()
			body, _ := c.GetRawData()
			query := c.Request.URL.Query()

			// Convert query parameters to a map
			queryParams := make(map[string]string)
			for key := range query {
				queryParams[key] = query.Get(key)
			}

			// Convert request headers to a map (this now includes Accept-Language)
			headers := make(map[string]string)
			for key := range c.Request.Header {
				headers[key] = c.Request.Header.Get(key)
			}

			statusCode, response := serveHTTPHandler(ctx, route.Handler, string(body), queryParams, headers)
			c.String(statusCode, response)
		})
	}

	if port == "" {
		port = "8080"
	}
	log.Printf("[Server] Server running on port %s", port)
	s.router.Run(":" + port)
}

// optionsResponse generates the CORS response for OPTIONS requests
func (s *Server) optionsResponse() events.APIGatewayProxyResponse {
	allowedMethods := s.getAllowedMethods()
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": strings.Join(allowedMethods, ", "),
		// Include Accept-Language in allowed headers
		"Access-Control-Allow-Headers": "Content-Type, Authorization, Accept-Language",
	}
	log.Printf("[Lambda] Responding to OPTIONS with headers: %+v", headers)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    headers,
	}
}

// serveHTTPHandler converts local requests to Lambda handler signature,
// now also including the headers.
func serveHTTPHandler(ctx context.Context, handler HandlerFunction, body string, query map[string]string, headers map[string]string) (int, string) {
	req := events.APIGatewayProxyRequest{
		Body:                  body,
		QueryStringParameters: query,
		Headers:               headers,
	}

	response, err := handler(ctx, req)
	if err != nil {
		log.Printf("Handler error: %v", err)
		return http.StatusInternalServerError, "Internal Server Error"
	}

	return response.StatusCode, response.Body
}

// setupCORS dynamically generates the CORS configuration,
// including Accept-Language in the allowed headers.
func (s *Server) setupCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins
		AllowMethods:     s.getAllowedMethods(),
		AllowHeaders:     []string{"Content-Type", "Authorization", "Accept-Language"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

// getAllowedMethods returns the dynamically tracked allowed methods
func (s *Server) getAllowedMethods() []string {
	methods := []string{string(OPTIONS)} // OPTIONS is always allowed
	for method := range s.allowedMethods {
		methods = append(methods, string(method))
	}
	slices.Sort(methods) // Sort for consistency
	return methods
}
