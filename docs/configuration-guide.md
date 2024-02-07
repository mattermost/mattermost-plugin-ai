# Configuration guide

The Mattermost AI Plugin is configured via **System Console** ➡️ **Mattermost AI Plugin**.

- [OpenAI](#openai)
- [Anthropic (Claude)](#anthropic-claude)
- [Azure OpenAI](#azure-openai)
- [Ask Sage](#ask-sage)
- [OpenAI Compatible](#openai-compatible)

## OpenAI

1. Obtain an [OpenAI API key](https://platform.openai.com/account/api-keys)
1. In the **AI Service** dropdown, select **OpenAI**
1. In the **API Key** field, enter your OpenAI API key
1. In the **Default Model** field, enter a model name that you can access with your API key (e.g. `gpt-4` or `gpt-3.5-turbo`)
1. **(Optional)** If you're part of multiple organizations on OpenAI and prefer not to use the default one, enter your specific **Organization ID**. This step ensures API usage and billing are attributed to your selected organization.

## Anthropic (Claude)

1. Obtain an [Anthropic API key](https://console.anthropic.com/account/keys)
1. In the **AI Service** dropdown, select **Anthropic**
1. In the **API Key** field, enter your OpenAI API key
1. In the **Default Model** field, enter a model name that you can access with your API key (e.g., `claude-v1`)

## [Azure OpenAI](https://learn.microsoft.com/en-us/azure/ai-services/openai/overview)

To learn about accessing OpenAI LLMs via Microsoft Azure, [refer to official documentation](https://learn.microsoft.com/en-us/azure/ai-services/openai/overview).

This requires functions to be supported, which has limited availability (e.g., model version `0613` with API `2023-07-01-preview`). Please refer to [official documentation](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/models) for the latest availability information.

1. Obtain [access to Azure OpenAI](https://learn.microsoft.com/en-us/azure/ai-services/openai/overview#how-do-i-get-access-to-azure-openai)
1. In Azure, create a new OpenAI resource
1. Deploy the model resource on Azure (don't select auto-update on your deployed model as it will auto-downgrade it to `0301` within a few minutes)
1. On Mattermost, in the **AI Service** dropdown, select **OpenAI Compatible**
1. In the **API URL** field, enter the URL to your Azure resource
1. In the **API Key** field, enter your Azure resource API key
1. In the **Default Model** field, enter the model name (e.g., `gpt-35-turbo`)

## Ask Sage

Ask Sage is currently supported as an experimental feature. Token-based security is not yet available via the Ask Sage API, and server configuration would require securing the Mattermost server configuration data store, which will contain username and password in plaintext.

The Ask Sage API does not yet support streaming, so there is less feedback to Mattermost users on intermediate information.

1. Obtain [access to Ask Sage](https://asksage.ai)
1. In the **AI Service** dropdown, select **Ask Sage**
1. In the **Username** field, enter your Ask Sage username
1. In the **Password** field, enter your Ask Sage password
1. In the **Default Model** field, enter an OpenAI model name (e.g., `gpt-4` or `gpt-3.5-turbo`)
1. enter the account's `username` and `password` on the System Console page and set the default model such as `gpt-4` or `gpt-3.5-turbo`.

## OpenAI Compatible

The Mattermost AI Plugin can support any LLM provider that is OpenAI-compatible such as [LocalAI](https://github.com/go-skynet/LocalAI).

1. Deploy your model on LocalAI or equivalent
1. In the **AI Service** dropdown, select **OpenAI Compatible**
1. In the **API URL** field, enter the URL to LocalAI
1. In the **API Key** field, enter your OpenAI API key
1. In the **Default Model** field, enter the model name
