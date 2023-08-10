import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from '@mattermost/types/lib/store';
import {FileInfo} from '@mattermost/types/files';

import {manifest} from '@/manifest';

import {LLMBotPost} from './components/llmbot_post';
import PostMenu from './components/post_menu';
import EditorMenu from './components/editor_menu';
import IconThreadSummarization from './components/assets/icon_thread_summarization';
import IconReactForMe from './components/assets/icon_react_for_me';
import NotebookPreview from './components/notebook_preview/notebook_preview';
import {doReaction, doSummarize, doTranscribe} from './client';
import {BotUsername} from './constants';

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: any, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerPostTypeComponent('custom_llmbot', LLMBotPost);
        if (registry.registerPostActionComponent) {
            registry.registerPostActionComponent(PostMenu);
        } else {
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</>, (postId: string) => {
                const state = store.getState();
                const team = state.entities.teams.teams[state.entities.teams.currentTeamId];
                window.WebappUtils.browserHistory.push('/' + team.name + '/messages/@' + BotUsername);
                doSummarize(postId);
            });
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconThreadSummarization/></span>{'Summarize Meeting Audio'}</>, doTranscribe);
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconReactForMe/></span>{'React for me'}</>, doReaction);
        }
        if (registry.registerPostEditorActionComponent) {
            registry.registerPostEditorActionComponent(EditorMenu);
        }
        registry.registerFilePreviewComponent((fileInfo: FileInfo) => fileInfo.extension === "ipynb", NotebookPreview);
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void
        WebappUtils: any
    }
}

window.registerPlugin(manifest.id, new Plugin());
