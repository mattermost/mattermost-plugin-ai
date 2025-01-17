# Mattermost Copilot Plugin [![Download Latest Master Build](https://img.shields.io/badge/Download-Latest%20Master%20Build-blue)](https://github.com/mattermost/mattermost-plugin-ai/releases/tag/latest-master)

The Mattermost Copilot Plugin integrates AI capabilities directly into your Mattermost workspace, supporting both local (self-hosted) and third-party (vendor-hosted) Large Language Models (LLMs). 

**Note**: The latest master build is experimental and may contain unstable features.

![The Mattermost Copilot AI Plugin is an extension for mattermost that provides functionality for local and third-party LLMs](https://github.com/mattermost/mattermost-plugin-ai/assets/2040554/6a787ff6-013d-4492-90ce-54aa7a292a4a)

## Features

- Support for multiple LLM providers including OpenAI, Anthropic, and custom endpoints
- AI-powered chat assistance in channels and threads
- Context-aware responses based on conversation history
- Customizable bot personas and system prompts
- Enterprise features for team collaboration

## Installation

1. Download the latest release from our [releases page](https://github.com/mattermost/mattermost-plugin-ai/releases)
2. Upload the plugin through the Mattermost System Console
3. Configure your desired LLM provider settings
4. Enable the plugin to start using AI features

### System Requirements

- Mattermost Server versions:
  - v9.6 or later (recommended)
  - v9.5.2+ (ESR)
  - v9.4.4+
  - v9.3.3+
  - v8.1.11+ (ESR)
- PostgreSQL database
- Network access to your chosen LLM provider

## Development

To set up a development environment:

1. Clone the repository
2. Install dependencies:
   ```bash
   make deps
   ```
3. Build the plugin:
   ```bash
   make dist
   ```
4. Deploy to your local Mattermost instance:
   ```bash
   make deploy
   ```

## Community & Support

Join our active community:
- [AI-Exchange Channel](https://community.mattermost.com/core/channels/ai-exchange) for discussions
- [Discourse Forum](https://forum.mattermost.com/c/openops-ai/40) for questions and updates

## Configuration

After installation, you'll need to configure the plugin through the System Console:

1. Navigate to **System Console > Plugins > Mattermost Copilot**
2. Configure your LLM provider settings:
   - API keys
   - Model selection
   - Token limits
   - Custom instructions
3. Set up access controls:
   - Channel access levels
   - User permissions
   - Team configurations

For detailed configuration instructions, see the [Mattermost Product Documentation](https://docs.mattermost.com/configure/enable-copilot.html).

## Development

### Prerequisites

- Go 1.20+
- Node.js 16.x+
- Make
- [Mattermost Server](https://mattermost.com/download/) with PostgreSQL
- Access to an LLM provider (OpenAI, Anthropic, etc.)

### Local Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/mattermost/mattermost-plugin-ai.git
   cd mattermost-plugin-ai
   ```

2. Install dependencies:
   ```bash
   make deps
   ```

3. Build the plugin:
   ```bash
   make dist
   ```

4. Deploy to your local Mattermost instance:
   ```bash
   make deploy
   ```

5. Enable the plugin in System Console and configure your LLM provider settings

### Development Tips

- Use `make watch` for hot reloading during development
- Run `make check-style` to verify code style
- Execute `make test` to run the test suite

## Contributing

We welcome contributions! To get started:

1. Review our [contributor guidelines](./.github/CONTRIBUTING.md)
2. Fork the repository and create a feature branch
3. Make your changes following our coding standards
4. Submit a pull request with a clear description of your changes

## Release Process

To create a new release:

- Patch: `make patch`
- Minor: `make minor`
- Major: `make major`

For release candidates, append `-rc` to the commands (e.g., `make patch-rc`).

## License

This repository is licensed under [Apache-2](./LICENSE), except for the [server/enterprise](server/enterprise) directory which is licensed under the [Mattermost Source Available License](LICENSE.enterprise). See [Mattermost Source Available License](https://docs.mattermost.com/overview/faq.html#mattermost-source-available-license) to learn more.
