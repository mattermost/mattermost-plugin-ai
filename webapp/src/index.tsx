import {Store, Action} from 'redux';

import {GlobalState} from '@mattermost/types/lib/store';

import {manifest} from '@/manifest';

import {LLMBotPost} from './components/llmbot_post';
import {doReaction} from './client';

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: any, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerPostDropdownMenuAction('React for me', doReaction);
        registry.registerPostTypeComponent('custom_llmbot', LLMBotPost);
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
