import {Store, Action} from 'redux';

import {GlobalState} from '@mattermost/types/lib/store';
import {ClientError} from '@mattermost/client';
import {Client4} from 'mattermost-redux/client';

import {manifest} from '@/manifest';

import {PluginRegistry} from '@/types/mattermost-webapp';

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: any, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerPostDropdownMenuAction('React for me', doReaction);
    }
}

async function doReaction(postid: string) {
    const url = '/plugins/summarize/react/' + postid;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
