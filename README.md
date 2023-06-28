# Mattermost AI Plugin

![Screenshot](/img/banner_fun.png)

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

# Table of Contents

- [Mattermost AI Plugin](#mattermost-ai-plugin)
- [Table of Contents](#table-of-contents)
  - [Background](#background)
  - [Install Mattermost + `mattermost-plugin-ai`](#install-mattermost--mattermost-plugin-ai)
    - [On existing Mattermost server](#on-existing-mattermost-server)
    - [Local Development](#local-development)
    - [Gitpod Demo](#gitpod-demo)
  - [Usage](#usage)
    - [Streaming Conversation](#streaming-conversation)
    - [Thread Summarization](#thread-summarization)
    - [Answer questions about Threads](#answer-questions-about-threads)
    - [Chat anywhere](#chat-anywhere)
    - [Create meeting summary](#create-meeting-summary)
    - [React for me](#react-for-me)
    - [RLHF Feedback Collection](#rlhf-feedback-collection)
  - [Community Resources](#community-resources)
    - [OpenOps \& AI](#openops--ai)
    - [Mattermost](#mattermost)
  - [Contributing](#contributing)
  - [License](#license)

## Background

**🚀 Join the ["AI Exchange" community server channel](https://community.mattermost.com/core/channels/ai-exchange) where Mattermost's open source community is sharing the latest AI resources and innovations!**

The Mattermost AI plugin adds functionality to use a wide variety of open source self hosted and vendor-hosted LLMs like OpenAI and GPT4All within Mattermost. 

This plugin is currently experimental. Contributions and suggestions are welcome, [see below](#contributing)! 

The Mattermost AI Plugin is used as part of the [Mattermost OpenOps](https://openops.mattermost.com) framework for responsible development of AI-enhanced workflows with the ability to maintain full data control and data portability across different AI backends. 

## Install Mattermost + `mattermost-plugin-ai`

### On existing Mattermost server

1. Download the latest release from https://github.com/mattermost/mattermost-plugin-ai/releases
1. Upload it to your server via System Console > Plugin Management.
1. Enable the plugin and configure the settings as desired. 

### Local Development

1. Clone and enter this repository:
    ```bash
    git clone https://github.com/mattermost/mattermost-plugin-ai && cd mattermost-plugin-ai
    ```
1. Install `mattermost-plugin-ai` on Mattermost:
    ```bash
    `MM_SERVICESETTINGS_SITEURL=http://localhost:8065 MM_ADMIN_USERNAME=<YOUR_USERNAME> MM_ADMIN_PASSWORD=<YOUR_PASSWORD> make deploy`
    ```
1. Access Mattermost and configure the plugin:
  * Open Mattermost at `http://localhost:8065`
  * Select **View in Browser**
  * In the top left Mattermost menu, click **System Console** ➡️ [**Mattermost AI Plugin**](http://localhost:8065/admin_console/plugins/plugin_mattermost-ai)
  * Enable the plugin and configure plugin settings as desired.

### Gitpod Demo

See our demo setup [OpenOps](https://github.com/mattermost/openops#install-openops-mattermost--mattermost-plugin-ai) for an easy to start demo. 

## Usage

### Streaming Conversation

Chat with an LLM right inside the Mattermost interface. Answer are streamed so you don't have to wait:

![Summarizing Thread](/img/summarize_thread.gif)

### Thread Summarization
Use the post menu or the `/summarize` command to get a summary of the thread in a Direct Message from the AI Bot:

![Summarizing Thread](/img/summarize_button.gif)

### Answer questions about Threads
Respond to the bot post to ask follow up questions:

![Thread Interrogation](/img/thread_interrogation.png)

### Chat anywhere
Just mention @ai anywhere in Mattermost to ask it to respond. It will be given the context of the thread you are participating in:

![Chat anywhere](/img/chat_anywhere.png)

### Create meeting summary
Create meeting summaries! Designed to work with the [calls plugin's](https://github.com/mattermost/mattermost-plugin-calls) recording feature.

![Meeting Summary](/img/meeting_summary.png)

### React for me
Just for fun! Use the post menu to ask the bot to react to the post. It will try to pick an appropriate reaction.

https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/5282b066-86b5-478d-ae10-57c3cb3ba038

### RLHF Feedback Collection
Bot posts have 👍 👎 icons that collect user feedback. The idea would be to use this as input for RLHF fine tuning or prompt development.

## Community Resources 

### OpenOps & AI
- [OpenOps General Discussion on Mattermost Forum](https://forum.mattermost.com/c/openops-ai/40) 
- [OpenOps Troubleshooting Discussion on Mattermost Forum](https://forum.mattermost.com/t/openops-ai-troubleshooting/15942/)
- [OpenOps Q&A on Mattermost Forum](https://forum.mattermost.com/t/openops-ai-faqs/16287)
- [OpenOps "AI Exchange" channel on Mattermost Community server](https://community.mattermost.com/core/channels/ai-exchange) (for Mattermost community interested in AI)
- [OpenOps Discord Server](https://discord.gg/VqzB4bz6) (for AI community interested in Mattermost) 

### Mattermost
- [Mattermost Troubleshooting Discussion on Mattermost Forum](https://forum.mattermost.com/c/trouble-shoot/16)
- [Mattermost "Peer-to-peer Help" channel on Mattermost Community server](https://community.mattermost.com/core/channels/peer-to-peer-help)

## Contributing

Thank you for your interest in contributing to our open source project! ❤️ To get started, please read the [contributor guidelines](./CONTRIBUTING.md) for this repository.

## License

This repository is licensed under [Apache-2](./LICENSE).
