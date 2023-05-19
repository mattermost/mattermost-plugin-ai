# Mattermost LLM Extensions Plugin

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://github.com/azigler/mattermost-plugin-summarize)

The LLM extensions plugin adds functionality around the use and development of LLMs like GPT-3.5 / 4 and hugging face models within Mattermost. 

Currently at the experimental phase of development. Contributions and suggestions welcome! 

## Conversation

Chat with an LLM right inside the Mattermost interface. Answer are streamed so you don't have to wait:

https://github.com/crspeller/mattermost-plugin-summarize/assets/3191642/f375f1a2-61bf-4ae1-839b-07e44461809b

## Thread Summarization
Use the post menu or the `/summarize` command to get a summary of the thread in a DM:
![Summarizing Thread](/img/summarize_thread.png)

## Answer questions about Threads
Respond to the bot post to ask follow up questions:

https://github.com/crspeller/mattermost-plugin-summarize/assets/3191642/6fed05e2-ee68-40db-9ee4-870c61ccf5dd

## Chat anywhere
Just mention @llmbot anywhere in Mattermost to ask it to respond. It will be given the context of the thread you are participating in:
![Bot Chat](/img/mention_bot.png)

## React for me
Just for fun! Use the post menu to ask the bot to react to the post. It will try to pick an appropriate reaction.

https://github.com/crspeller/mattermost-plugin-summarize/assets/3191642/5282b066-86b5-478d-ae10-57c3cb3ba038

## RLHF Feedback Collection
Bot posts will have :+1: :-1: icons for collecting feedback. The idea would be to use this as input for RLHF fine tuning.


## Installation

1. Go the releases page and download the latest release.
2. On your Mattermost, go to System Console -> Plugin Management and upload it. [More Details](https://docs.mattermost.com/administration/plugins.html#plugin-uploads)
3. Enable the plugin and configure plugin settings as desired.

## Configuration

Lots of unfinished work in the system console settings. For now all you need to do is input and OpenAI API Key and configure allowed teams/users as desired. More options and the ability to use local LLMs is on the roadmap.
