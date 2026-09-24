# gin-multi-server

`gin-multi-server` lets the same handlers run on AWS Lambda or a traditional local Gin server. Lambda mode supports both API Gateway REST API (payload v1) and HTTP API (payload v2) events.

## Features

- **Dual Mode Support**: Runs on AWS Lambda or as a local server without modification.
- **Dynamic Routing**: Easily define and mount routes with custom handlers.
- **CORS Configuration**: Automatically sets up CORS headers based on registered routes.
- **Lightweight and Extensible**: Designed as a blueprint for extensibility.
- **Simple API**: Minimal learning curve for integrating into your project.

## Installation

Install the package using `go get`:

```bash
go get github.com/GEMSDEV-mx/gin-multi-server
```

## Usage

### Example

Here's a quick example of how to use `gin-lambda-server`:

```go
package main

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	server "github.com/GEMSDEV-mx/gin-multi-server"
)

func main() {
	s := server.NewServer()

	// Define a handler
	handler := func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "Hello, World!",
		}, nil
	}

	// Mount routes
	s.MountEndpoint(server.GET, "/hello", handler)

	// Start the server
	s.Serve("8080")
}
```

### Running Locally

Set the `AWS_LAMBDA_FUNCTION_NAME` environment variable to an empty string or leave it unset to run the server in local mode. Pass an empty port to use the default, 8080.

```bash
go run main.go
```

### Running on AWS Lambda

Deploy the application as an AWS Lambda function. The server automatically switches to Lambda mode if `AWS_LAMBDA_FUNCTION_NAME` is set in the environment.

## API

### `NewServer()`

Creates a new instance of the server.

### `MountEndpoint(method Method, path string, handler HandlerFunction)`

Mounts a new route to the server.

- **`method`**: The HTTP method (e.g., `GET`, `POST`).
- **`path`**: The URL path (e.g., `/example`).
- **`handler`**: A function matching the `HandlerFunction` signature.

### `Serve(port string)`

Starts the server. Automatically detects whether to run in Lambda mode or as a local server.

## Extensibility

This package is designed as a blueprint. You can extend its functionality to support other platforms by modifying the `Serve()` method or adding new integrations.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with love using the Gin framework and AWS Lambda Go SDK.
