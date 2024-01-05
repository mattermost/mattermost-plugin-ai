# Developer setup guide

1. If you don't yet have a Mattermost server for development, first follow the [developer setup guide](https://developers.mattermost.com/contribute/server/developer-setup/).

2. Clone and enter this repository:

    ```bash
    git clone https://github.com/mattermost/mattermost-plugin-ai && cd mattermost-plugin-ai
    ```

3. Install this plugin on Mattermost:

    ```bash
    MM_SERVICESETTINGS_SITEURL=http://localhost:8065 MM_ADMIN_USERNAME=<YOUR_USERNAME> MM_ADMIN_PASSWORD=<YOUR_PASSWORD> make deploy
    ```

4. Access Mattermost and configure the plugin:

- Open Mattermost at `http://localhost:8065`
- Select **View in Browser**
- In the top left Mattermost menu, click **System Console** ➡️ [**Mattermost AI Plugin**](http://localhost:8065/admin_console/plugins/plugin_mattermost-ai)
- Enable the plugin and configure plugin settings as desired. See [Configuration](https://github.com/mattermost/mattermost-plugin-ai#configuration).
