# gin-lambda-server

`gin-lambda-server` is a Go package that provides a blueprint for creating servers that can seamlessly run on both AWS Lambda and a traditional local server. This package is built on top of the Gin framework and includes features like dynamic routing, CORS handling, and more, making it easy to develop, test, and deploy applications across multiple environments.

## Features

- **Dual Mode Support**: Runs on AWS Lambda or as a local server without modification.
- **Dynamic Routing**: Easily define and mount routes with custom handlers.
- **CORS Configuration**: Automatically sets up CORS headers based on registered routes.
- **Lightweight and Extensible**: Designed as a blueprint for extensibility.
- **Simple API**: Minimal learning curve for integrating into your project.

## Installation

Install the package using `go get`:

```bash
go get github.com/yourusername/gin-lambda-server
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
	"github.com/yourusername/gin-lambda-server"
)

func main() {
	server := gin_lambda_server.NewServer()

	// Define a handler
	handler := func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "Hello, World!",
		}, nil
	}

	// Mount routes
	server.MountEndpoint(gin_lambda_server.GET, "/hello", handler)

	// Start the server
	server.Serve()
}
```

### Running Locally

Set the `AWS_LAMBDA_FUNCTION_NAME` environment variable to an empty string or leave it unset to run the server in local mode. Use the `PORT` environment variable to specify the port (default is 8080).

```bash
PORT=8080 go run main.go
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

### `Serve()`

Starts the server. Automatically detects whether to run in Lambda mode or as a local server.

## Extensibility

This package is designed as a blueprint. You can extend its functionality to support other platforms by modifying the `Serve()` method or adding new integrations.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with love using the Gin framework and AWS Lambda Go SDK.
