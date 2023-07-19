# Mattermost AI Plugin

![Screenshot](/img/banner_fun.png)

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

# Table of Contents

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
  - [Personalisation](#personalisation)
  - [User lookup (OpenAI exclusive)](#user-lookup-openai-exclusive)
  - [Channel posts lookup (OpenAI exclusive)](#channel-posts-lookup-openai-exclusive)
  - [GitHub integration (OpenAI exclusive, requires GitHub plugin)](#github-integration-openai-exclusive-requires-github-plugin)
  - [React for me](#react-for-me)
  - [RLHF Feedback Collection](#rlhf-feedback-collection)
- [Supported Backends](#supported-backends)
  - [OpenAI (recommended)](#openai-recommended)
  - [Anthropic](#anthropic)
  - [Azure OpenAI](#azure-openai)
  - [OpenAI Compatable](#openai-compatable)
  - [Ask Sage](#ask-sage)
- [Community Resources](#community-resources)
  - [AI](#ai)
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
  * Enable the plugin and configure plugin settings as desired. See [Supported Backends](#supported-backends).

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

### Personalisation
Context such as the current channel and user are supplied to the LLM when you make requests. Allowing customization of responses.
![Personalisation](/img/personalization.png)

### User lookup (OpenAI exclusive) 
The LLM can lookup other users on the system if you ask about them.

OpenAI exclusive for now since it requires the function API.

### Channel posts lookup (OpenAI exclusive)
You can ask about other channels and the LLM can ingest posts from that channel. For example you can ask it to summarize the last few posts in a channel. Note, depending on if you have CRT enabled this may not behave as you expect.
![Personalisation](/img/posts_lookup.png)

OpenAI exclusive for now since it requires the function API.

### GitHub integration (OpenAI exclusive, requires GitHub plugin)
The LLM can attempt to lookup specific GitHub issues. For example you can paste a GitHub link into the chat and ask questions about it. Only the title and description for now.
![Github](/img/github.png)

OpenAI exclusive for now since it requires the function API.

### React for me
Just for fun! Use the post menu to ask the bot to react to the post. It will try to pick an appropriate reaction.

https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/5282b066-86b5-478d-ae10-57c3cb3ba038

### RLHF Feedback Collection
Bot posts have 👍 👎 icons that collect user feedback. The idea would be to use this as input for RLHF fine tuning or prompt development.

## Supported Backends

All backends are configured in the system console settings page for the plugin. 
Make sure to select your preferred backend under `AI Large Language Model service` on the system console page after configuring. 

### OpenAI (recommended)
To set this up get an OpenAI api key. You will need to sign up for an account if you don't have one already. You can go to https://platform.openai.com/account/api-keys to create a new one. 

Configure the key in the system console and add a model like `gpt-4` (better) or `gpt-3.5-turbo` (faster and cheaper)

### Anthropic
You will need to have an invite to the Anthropic API. 

If you do you can create an APi key here: https://console.anthropic.com/account/keys

Configure the API key in the system console and configure a default model like `claude-v1`.

### Azure OpenAI
You will need to ask Azure to enable OpenAI in your Azure account before you can use this API.

Once you have been approved, you can create a new OpenAI resource. With the resource created you get access to the API key and the endpoint url clicking in keys and endpoints option of the menu.

Finally you have to deploy the model that you are going to use, normally gpt-35-turbo, clicking in "Model deployments", and managing the models from there.

Configure the API key and the endpoint url for OpenAI Compatible in the system console and configure a default model like `gpt-35-turbo`.

### OpenAI Compatable
Can support any backend that is OpenAI compatable such as [LocalAI](https://github.com/go-skynet/LocalAI) which we use in the [OpenOps](https://github.com/mattermost/openops) demo.

### Ask Sage
If you can to use the OpenAI api directly, it is recommended you do that. Ask Sage does not support response streaming leading to a worse user experience. API tokens have not been implemented by Ask Sage therefore the Ask Sage integration requires username and password stored in plaintext in the server configuration. Hopefully these limitations will be resolved.

To configure enter your username and password on the system console page and set the default model such as `gpt-4` or `gpt-3.5-turbo`.

## Community Resources 

### AI
- ["AI Exchange" channel on Mattermost Community server](https://community.mattermost.com/core/channels/ai-exchange) (for Mattermost community interested in AI)
- [OpenOps General Discussion on Mattermost Forum](https://forum.mattermost.com/c/openops-ai/40) 
- [OpenOps Troubleshooting Discussion on Mattermost Forum](https://forum.mattermost.com/t/openops-ai-troubleshooting/15942/)
- [OpenOps Q&A on Mattermost Forum](https://forum.mattermost.com/t/openops-ai-faqs/16287)

### Mattermost
- [Mattermost Troubleshooting Discussion on Mattermost Forum](https://forum.mattermost.com/c/trouble-shoot/16)
- [Mattermost "Peer-to-peer Help" channel on Mattermost Community server](https://community.mattermost.com/core/channels/peer-to-peer-help)

## Contributing

Thank you for your interest in contributing to our open source project! ❤️ To get started, please read the [contributor guidelines](./CONTRIBUTING.md) for this repository.

## License

This repository is licensed under [Apache-2](./LICENSE).
