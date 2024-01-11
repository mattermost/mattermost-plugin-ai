# Developer setup guide

1. If you don't yet have a Mattermost instance for development, first follow the [Mattermost developer setup guide](https://developers.mattermost.com/contribute/server/developer-setup/). For additional information about Mattermost plugin development, check out the [plugin developer setup guide](https://developers.mattermost.com/integrate/plugins/developer-setup/).

1. Clone and enter this repository:

    ```bash
    git clone https://github.com/mattermost/mattermost-plugin-ai && cd mattermost-plugin-ai
    ```

1. Install Mattermost AI Plugin on Mattermost by following the [plugin developer workflow documentation](https://developers.mattermost.com/integrate/plugins/developer-workflow/) or using this command:

    ```bash
    MM_SERVICESETTINGS_SITEURL=http://localhost:8065 MM_ADMIN_USERNAME=<YOUR_USERNAME> MM_ADMIN_PASSWORD=<YOUR_PASSWORD> make deploy
    ```

1. Access Mattermost and configure the Mattermost AI Plugin:

   1. Log in to Mattermost as an administrator
   1. Upload the Mattermost AI Plugin via **System Console** ➡️ **Plugin Management**
   1. Enable the Mattermost AI Plugin via **System Console** ➡️ **Mattermost AI Plugin**.

1. Follow the [configuration guide](./docs/configuration-guide.md) to set up the Mattermost AI Plugin.
