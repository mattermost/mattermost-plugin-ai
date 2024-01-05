# Mattermost AI Plugin (`mattermost-plugin-ai`)

![Screenshot](/img/banner_fun.png)

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

## Table of Contents

- [Mattermost AI Plugin (`mattermost-plugin-ai`)](#mattermost-ai-plugin-mattermost-plugin-ai)
  - [Table of Contents](#table-of-contents)
  - [Background](#background)
  - [Install](#install)
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
  - [Configuration](#configuration)
    - [OpenAI (recommended)](#openai-recommended)
    - [Anthropic](#anthropic)
    - [Azure OpenAI](#azure-openai)
    - [OpenAI Compatable](#openai-compatable)
    - [Ask Sage](#ask-sage)
  - [Contributing](#contributing)
  - [License](#license)

## Background

**🚀 Join the ["AI Exchange" community server channel](https://community.mattermost.com/core/channels/ai-exchange) where Mattermost's open source community is sharing the latest AI resources and innovations!**

The Mattermost AI plugin adds functionality to use a wide variety of open source self hosted and vendor-hosted LLMs like OpenAI and GPT4All within Mattermost.

This plugin is currently experimental. Contributions and suggestions are welcome, [see below](#contributing)!

The Mattermost AI Plugin is used as part of the [Mattermost OpenOps](https://openops.mattermost.com) framework for responsible development of AI-enhanced workflows with the ability to maintain full data control and data portability across different AI backends.

## Install

1. Download the latest release from <https://github.com/mattermost/mattermost-plugin-ai/releases>
1. Upload it to your server via **System Console** ➡️ **Plugin Management**.
1. Enable the plugin and configure the settings as desired. See [Configuration](#configuration).

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

<https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/5282b066-86b5-478d-ae10-57c3cb3ba038>

## Configuration

All backends are configured in the system console settings page for the plugin.
Make sure to select your preferred backend under `AI Large Language Model service` on the system console page after configuring.

### OpenAI (recommended)

To set this up get an OpenAI api key. You will need to sign up for an account if you don't have one already. You can go to <https://platform.openai.com/account/api-keys> to create a new one.

Configure the key in the system console and add a model like `gpt-4` (better) or `gpt-3.5-turbo` (faster and cheaper)

### Anthropic

You will need to have an invite to the Anthropic API.

If you do you can create an APi key here: <https://console.anthropic.com/account/keys>

Configure the API key in the system console and configure a default model like `claude-v1`.

### Azure OpenAI

You will need to ask Azure to enable OpenAI in your Azure account before you can use this API.

This api requires functions to be supported, and they are for now only on models version `0613` with API `2023-07-01-preview`. They are avaiable on limited datacenters right now. For moment of writing this docs avaiable regions for gpt-35-turbo v0613 are: Canada East, East US, France Central, Japan East, North Central US, UK South. More info in [azure docs](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/models)

Once you have been approved, you can create a new OpenAI resource. With the resource created you get access to the API key and the endpoint url clicking in keys and endpoints option of the menu.

Finally you have to deploy the model that you are going to use, normally gpt-35-turbo, clicking in "Model deployments", and managing the models from there. (TIP: don't select auto-update on your deployed model, it will auto-downgrade it to 0301 within about 5-10 minutes)

Configure the API key and the endpoint url for OpenAI Compatible in the system console and configure a default model like `gpt-35-turbo`.

### OpenAI Compatable

Can support any backend that is OpenAI compatable such as [LocalAI](https://github.com/go-skynet/LocalAI) which we use in the [OpenOps](https://github.com/mattermost/openops) demo.

### Ask Sage

Ask Sage is currently supported as an experimental stage feature. Token-based security is not yet available via the Ask Sage API, and server configuration would require securing the Mattermost server configuration data store, which will contain username and password in plaintext.

To configure, you need to purchase a commercial account from [https://asksage.ai](https://asksage.ai), enter the account's `username` and `password` on the System Console page and set the default model such as `gpt-4` or `gpt-3.5-turbo`.

The Ask Sage API doesn't yet support streaming, so there is less feedback to Mattermost users on intermediate information.

## Contributing

Thank you for your interest in contributing to our open source project! ❤️

How to get started:

1. Read the [contributor guidelines](./CONTRIBUTING.md) for this repository.
1. Follow [this plugin's developer setup guide](./docs/developer-setup-guide.md) to set up your development environment.
1. Check out the [Help Wanted ticket list](https://github.com/mattermost/mattermost-plugin-ai/labels/help%20wanted).
1. Join the [~AI-Exchange channel](https://community.mattermost.com/core/channels/ai-exchange) and explore the [Discourse forum](https://forum.mattermost.com/c/openops-ai/40).

## License

This repository is licensed under [Apache-2](./LICENSE).
