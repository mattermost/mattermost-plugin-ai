# Mattermost AI Plugin

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

![Screenshot](/img/mention_bot.png)

## Table of Contents

- [Mattermost AI Plugin](#mattermost-ai-plugin)
  - [Table of Contents](#table-of-contents)
  - [Background](#background)
  - [Install Mattermost + `mattermost-plugin-ai`](#install-mattermost--mattermost-plugin-ai)
  - [Usage](#usage)
    - [Conversation](#conversation)
    - [Thread Summarization](#thread-summarization)
    - [Answer questions about Threads](#answer-questions-about-threads)
    - [Chat anywhere](#chat-anywhere)
    - [React for me](#react-for-me)
    - [RLHF Feedback Collection](#rlhf-feedback-collection)
  - [Related Efforts](#related-efforts)
  - [Contributing](#contributing)
  - [License](#license)

## Background

**🚀 Join [Mattermost's AI discussion forums](https://forum.mattermost.com/c/ai-frameworks/40) and the ["AI Exchange" community server channel](https://community.mattermost.com/core/channels/ai-exchange) where Mattermost's open source community is sharing the latest AI resources and innovations!**

The LLM extensions plugin adds functionality to use and develop LLMs (like from OpenAI or Hugging Face) within Mattermost. 

This plugin is currently experimental. More options and the ability to use local LLMs is on the roadmap. Contributions and suggestions are welcome, [see below](#contributing)! 

## Install Mattermost + `mattermost-plugin-ai`

1. Clone and enter this repository:
  * `git clone https://github.com/mattermost/mattermost-plugin-ai && cd mattermost-plugin-ai`
2. Create a `.env.docker` file inside a `mattermost` folder.
   - You can adjust this accordingly based on the environment variables you see in the [`docker-compose.yml`](./docker-compose.yml) file.
3. Start the services:
  * `docker compose up -d`
4. Configure the Mattermost server from the init script:
  * `bash ./init.sh`
5. Install `mattermost-plugin-ai` on Mattermost from the command line:
  * `MM_SERVICESETTINGS_SITEURL=http://localhost:8065 MM_ADMIN_USERNAME=root MM_ADMIN_PASSWORD=<YOUR_PASSWORD> make deploy`
6. Access Mattermost and configure the plugin:
  * Open Mattermost at `http://localhost:8065`
  * Select **View in Browser**
  * Log in with the generated `root` credentials
  * In the top left Mattermost menu, click **System Console** ➡️ [**Mattermost AI Plugin**](http://localhost:8065/admin_console/plugins/plugin_mattermost-ai)
  * Enable the plugin and configure plugin settings as desired.

For example, you can configure your OpenAI API key and allowed teams/users as desired.

## Usage

### Conversation

Chat with an LLM right inside the Mattermost interface. Answer are streamed so you don't have to wait:

https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/f375f1a2-61bf-4ae1-839b-07e44461809b

### Thread Summarization
Use the post menu or the `/summarize` command to get a summary of the thread in a DM:

![Summarizing Thread](/img/summarize_thread.png)

### Answer questions about Threads
Respond to the bot post to ask follow up questions:

https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/6fed05e2-ee68-40db-9ee4-870c61ccf5dd

### Chat anywhere
Just mention @ai anywhere in Mattermost to ask it to respond. It will be given the context of the thread you are participating in:

![Bot Chat](/img/mention_bot.png)

### React for me
Just for fun! Use the post menu to ask the bot to react to the post. It will try to pick an appropriate reaction.

https://github.com/mattermost/mattermost-plugin-ai/assets/3191642/5282b066-86b5-478d-ae10-57c3cb3ba038

### RLHF Feedback Collection
Bot posts will have 👍 👎 icons that will later be used to collect feedback for RLHF fine tuning. The idea would be to use this as input for RLHF fine tuning.

## Related Efforts

Explore Mattermost's AI initiatives via the ["Welcome to Mattermost’s AI Community"](https://forum.mattermost.com/t/welcome-to-mattermosts-ai-community/16144?u=zigler) thread in the [AI forums](https://forum.mattermost.com/c/ai-frameworks/40).

## Contributing

Visit [our AI developer website](https://mattermost.github.io/mattermost-ai-site/) and check out Mattermost's [contributor guide](https://developers.mattermost.com/contribute/) to learn about contributing to our open source projects like this one.

## License

This repository is licensed under [Apache-2](./LICENSE).
