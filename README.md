# Mattermost AI Plugin

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

![Screenshot](/img/banner_fun.png)

# Table of Contents

- [Background](#background)
- [Install Mattermost + `mattermost-plugin-ai`](#install-mattermost-plugin-ai)
  - [On my existing MM server](#on-my-existing-mm-server)
  - [Local Development](#local-development)
  - [Gitpod Demo](#gitpod-demo)
- [Usage](#usage)
  - [Streaming Conversation](#streaming-conversation)
  - [Thread Summarization](#thread-summarization)
  - [Answer questions about Threads](#answer-questions-about-threads)
  - [Chat anywhere](#chat-anywhere)
  - [React for me](#react-for-me)
  - [RLHF Feedback Collection](#rlhf-feedback-collection)
- [Related Efforts](#related-efforts)
- [Contributing](#contributing)
- [License](#license)

## Background

**🚀 Join the ["AI Exchange" community server channel](https://community.mattermost.com/core/channels/ai-exchange) where Mattermost's open source community is sharing the latest AI resources and innovations!**

The Mattermost AI plugin adds functionality to use LLMs (like from OpenAI or Hugging Face) within Mattermost. 

This plugin is currently experimental. Contributions and suggestions are welcome, [see below](#contributing)! 

## Install Mattermost + `mattermost-plugin-ai`

### On my existing MM server

1. Download the latest release from https://github.com/mattermost/mattermost-plugin-ai/releases
1. Upload it to your server via System Console > Plugin Management.
1. Enable the plugin and configure the settings as desired. 

### Local Development

1. Clone and enter this repository:
  * `git clone https://github.com/mattermost/mattermost-plugin-ai && cd mattermost-plugin-ai`
1. Install `mattermost-plugin-ai` on Mattermost:
  * `MM_SERVICESETTINGS_SITEURL=http://localhost:8065 MM_ADMIN_USERNAME=<YOUR_USERNAME> MM_ADMIN_PASSWORD=<YOUR_PASSWORD> make deploy`
1. Access Mattermost and configure the plugin:
  * Open Mattermost at `http://localhost:8065`
  * Select **View in Browser**
  * In the top left Mattermost menu, click **System Console** ➡️ [**Mattermost AI Plugin**](http://localhost:8065/admin_console/plugins/plugin_mattermost-ai)
  * Enable the plugin and configure plugin settings as desired.

### Gitpod Demo

See out demo setup [OpenOps](https://github.com/mattermost/openops#install-openops-mattermost--mattermost-plugin-ai) for an easy to start demo. 

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

### React for me
Just for fun! Use the post menu to ask the bot to react to the post. It will try to pick an appropriate reaction.

https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/5282b066-86b5-478d-ae10-57c3cb3ba038

### RLHF Feedback Collection
Bot posts have 👍 👎 icons that collect user feedback. The idea would be to use this as input for RLHF fine tuning or prompt development.

## Related Efforts

Explore Mattermost's AI initiatives via the ["Welcome to Mattermost’s AI Community"](https://forum.mattermost.com/t/welcome-to-mattermosts-ai-community/16144?u=zigler) thread in the [AI forums](https://forum.mattermost.com/c/ai-frameworks/40).

## Contributing

To contribute to the project see [contributor guide](https://developers.mattermost.com/contribute/)

Join the [AI Exchange channel](https://community.mattermost.com/core/channels/ai-exchange) on our community server to discuss any questions.

Read our documentation about the [Developer Workflow](https://developers.mattermost.com/extend/plugins/developer-workflow/) and [Developer Setup](https://developers.mattermost.com/extend/plugins/developer-setup/) for more information about developing and extending plugins.

See the [issues](https://github.com/mattermost/mattermost-plugin-ai/issues) for what you can do to help.

## License

This repository is licensed under [Apache-2](./LICENSE).
