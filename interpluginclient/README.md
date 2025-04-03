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
request := interpluginclient.CompletionRequest{
    Prompt:          "Summarize this text: " + text,
    RequesterUserID: userID,
}

// Get the AI completion
response, err := client.Completion(request)
if err != nil {
    // Handle error
}

// Use the response
fmt.Println("AI response:", response)
```

### Advanced Usage

```go
// Create a client with custom parameters
params := interpluginclient.CompletionParameters{
    Model:             "gpt-4o",
    MaxGeneratedTokens: 500,
}

request := interpluginclient.CompletionRequest{
    Prompt:          "Explain quantum computing in simple terms",
    BotUsername:     "ai",  // Use a specific bot if configured
    RequesterUserID: userID,
    Parameters:      params.ToMap(),
}

// Set a custom timeout with context
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// Get the AI completion with context
response, err := client.CompletionWithContext(ctx, request)
if err != nil {
    // Handle error
}
```

## API Documentation

### Types

- `CompletionRequest`: Represents a request to the AI plugin for text completion
  - `Prompt`: The text prompt to send to the AI model
  - `BotUsername`: Which AI bot to use (optional, uses default bot if empty)
  - `RequesterUserID`: The user ID of the user requesting the completion
  - `Parameters`: Optional map for customizing the completion behavior

- `CompletionResponse`: Represents the response from an interplugin completion request
  - `Response`: The generated text from the AI model

- `CompletionParameters`: Helper for setting parameters like model and token limits
  - `Model`: Which specific model to use
  - `MaxGeneratedTokens`: Limits the maximum number of tokens generated in the response

- `Client`: The main client for communicating with the AI plugin

### Methods

- `NewClient(p *plugin.MattermostPlugin) (*Client, error)`: Creates a client from a plugin instance
- `Completion(req CompletionRequest) (string, error)`: Makes a completion request with default timeout (30 seconds)
- `CompletionWithContext(ctx context.Context, req CompletionRequest) (string, error)`: Makes a completion request with a custom context

### Constants

- `DefaultTimeout`: The default timeout for all requests to the AI plugin (30 seconds)
- `ErrAIPluginNotAvailable`: Error returned when the AI plugin is not available or not properly configured
