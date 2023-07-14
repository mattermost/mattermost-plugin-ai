import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from '@mattermost/types/lib/store';

import {manifest} from '@/manifest';

import {LLMBotPost} from './components/llmbot_post';
import IconAI from './components/icon_ai';
import IconReactForMe from './components/icon_react_for_me';
import IconThreadSummarization from './components/icon_thread_summarization';

// TODO: waiting for getting the backend implemented
// import AskAiInput from './components/ask_ai_input';
import {doReaction, doSummarize, doTranscribe} from './client';

const BotUsername = 'ai';

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: any, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerPostTypeComponent('custom_llmbot', LLMBotPost);
        if (registry.registerPostAction) {
            registry.registerPostAction('AI Actions', IconAI, [

                // TODO: waiting for getting the backend implemented
                // {text: <AskAiInput/>},
                {text: <><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</>, action: makeSummarizePost(store)},
                {text: <><span className='icon'><IconThreadSummarization/></span>{'Summarize Meeting Audio'}</>, action: doTranscribe},
                {text: <><span className='icon'><IconReactForMe/></span>{'React for me'}</>, action: doReaction},
            ]);
        } else {
            // TODO: waiting for getting the backend implemented
            // registry.registerPostDropdownMenuAction(<AskAiInput/>);
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</>, makeSummarizePost(store));
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconThreadSummarization/></span>{'Summarize Meeting Audio'}</>, doTranscribe);
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconReactForMe/></span>{'React for me'}</>, doReaction);
        }
    }
}

function makeSummarizePost(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return async function summarizePost(postid: string) {
        const state = store.getState();
        const team = state.entities.teams.teams[state.entities.teams.currentTeamId];
        window.WebappUtils.browserHistory.push('/' + team.name + '/messages/@' + BotUsername);

        await doSummarize(postid);
    };
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void
        WebappUtils: any
    }
}

window.registerPlugin(manifest.id, new Plugin());
