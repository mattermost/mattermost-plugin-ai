# AI Plugin Inter-Plugin Client

This package provides a client for interacting with the Mattermost AI plugin from other Mattermost plugins.

## Usage

### Basic Usage

```go
// Create a client from your plugin
client, err := interpluginclient.NewClient(&p.MattermostPlugin)
if err != nil {
    // Handle error
}

// Create a completion request
request := interpluginclient.SimpleCompletionRequest{
    Prompt:          "Summarize this text: " + text,
    RequesterUserID: userID,
}

// Get the AI completion
response, err := client.SimpleCompletion(request)
if err != nil {
    // Handle error
}

// Use the response
fmt.Println("AI response:", response)
```

### Advanced Usage

```go
request := interpluginclient.SimpleCompletionRequest{
    Prompt:          "Explain quantum computing in simple terms",
    BotUsername:     "ai",  // Use a specific bot if configured
    RequesterUserID: userID,
    Parameters:      map[string]any{},
}

// Set a custom timeout with context
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// Get the AI completion with context
response, err := client.SimpleCompletionWithContext(ctx, request)
if err != nil {
    // Handle error
}
```

## Security Notice

**Important**: The AI plugin's inter-plugin API endpoints do not perform permission checks. The calling plugin is responsible for:
- Verifying that the user specified in `RequesterUserID` has permission to use the AI features
- Ensuring the user has access to any data being sent to the AI model
- Implementing appropriate rate limiting or usage restrictions
- Validating that the request is authorized for the intended purpose

The AI plugin will process requests on behalf of any user ID provided by the calling plugin, so proper authorization checks are critical.

## API Documentation

### Types

- `CompletionRequest`: Represents a request to the AI plugin for text completion
  - `Prompt`: The text prompt to send to the AI model
  - `BotUsername`: Which AI bot to use (optional, uses default bot if empty)
  - `RequesterUserID`: The user ID of the user requesting the completion
  - `Parameters`: Optional map for customizing the completion behavior

- `Client`: The main client for communicating with the AI plugin

### Methods

- `NewClient(p *plugin.MattermostPlugin) (*Client, error)`: Creates a client from a plugin instance
- `SimpleCompletion(req SimpleCompletionRequest) (string, error)`: Makes a completion request with default timeout (30 seconds)
- `SimpleCompletionWithContext(ctx context.Context, req SimpleCompletionRequest) (string, error)`: Makes a completion request with a custom context

### Constants

- `DefaultTimeout`: The default timeout for all requests to the AI plugin (30 seconds)
