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

## Install

We recommend using Mattermost Server v9.6 or later for the best experience. Compatible Mattermost server versions include:

- v9.6 or later
- v9.5.2+ ([ESR](https://docs.mattermost.com/deploy/mattermost-changelog.html#release-v9-5-extended-support-release))
- v9.4.4+
- v9.3.3+
- v8.1.11+ ([ESR](https://docs.mattermost.com/deploy/mattermost-changelog.html))

See the [Mattermost Product Documentation](https://docs.mattermost.com/configure/enable-copilot.html) for details on installing, configuring, enabling, and using this Mattermost integration.

**Note**: Installation instructions assume you already have [Mattermost Server](https://mattermost.com/download/) installed and configured with [PostgreSQL](https://www.postgresql.org/).

## How to Release

To trigger a release, follow these steps:

1. **For Patch Release:** Run the following command:
    ```
    make patch
    ```
   This will release a patch change.

2. **For Minor Release:** Run the following command:
    ```
    make minor
    ```
   This will release a minor change.

3. **For Major Release:** Run the following command:
    ```
    make major
    ```
   This will release a major change.

4. **For Patch Release Candidate (RC):** Run the following command:
    ```
    make patch-rc
    ```
   This will release a patch release candidate.

5. **For Minor Release Candidate (RC):** Run the following command:
    ```
    make minor-rc
    ```
   This will release a minor release candidate.

6. **For Major Release Candidate (RC):** Run the following command:
    ```
    make major-rc
    ```
   This will release a major release candidate.


## Contributing

Interested in contributing to our open source project? Start by reviewing the [contributor guidelines](./.github/CONTRIBUTING.md) for this repository. See the [Developer Setup Guide](docs/developer-setup-guide.md) for details on setting up a Mattermost instance for development.

## License

This repository is licensed under [Apache-2](./LICENSE), except for the [server/enterprise](server/enterprise) directory which is licensed under the [Mattermost Source Available License](LICENSE.enterprise). See [Mattermost Source Available License](https://docs.mattermost.com/overview/faq.html#mattermost-source-available-license) to learn more.
