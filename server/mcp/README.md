# Model Control Protocol (MCP) Client

This package implements a client for the Model Control Protocol (MCP) that allows the AI plugin to access external tools provided by the MCP server.

## Usage

1. Configure MCP in your plugin configuration. Example:

```json
{
  "mcp": {
    "enabled": true,
    "baseURL": "https://your-mcp-server.com",
    "headers": {
      "Authorization": "Bearer your-token-here"
    }
  }
}
```

2. Tools provided by the MCP server will be automatically added to the AI plugin's tool store when enabled, making them available to the AI models during conversations.