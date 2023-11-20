import React from 'react';
import {Store, Action} from 'redux';
import styled from 'styled-components';

import {GlobalState} from '@mattermost/types/lib/store';

import {manifest} from '@/manifest';

import {LLMBotPost} from './components/llmbot_post';
import PostMenu from './components/post_menu';
import EditorMenu from './components/editor_menu';
import CodeMenu from './components/code_menu';
import IconThreadSummarization from './components/assets/icon_thread_summarization';
import IconReactForMe from './components/assets/icon_react_for_me';
import IconAI from './components/assets/icon_ai';
import RHS from './components/rhs/rhs';
import Config from './components/config/config';
import {doReaction, doSummarize, doTranscribe, getAIDirectChannel} from './client';
import {setOpenRHSAction} from './redux_actions';
import {BotUsername} from './constants';
import PostEventListener from './websocket';
import {setupRedux} from './redux';

type WebappStore = Store<GlobalState, Action<Record<string, unknown>>>

const StreamingPostWebsocketEvent = 'custom_mattermost-ai_postupdate';

const IconAIContainer = styled.span`
    filter: invert(1);
    border-radius: 50%;
    width: 28px;
    height: 28px;
    display: inline-flex;
    align-items: center;
    justify-items: center;
    margin-right: 5px;
    background-color: var(--center-channel-bg);
`;

const RHSTitleContainer = styled.span`
    display: flex;
    align-items: center;
`;

const RHSTitle = () => {
    return (
        <RHSTitleContainer>
            <IconAIContainer className='icon'>
                <IconAI/>
            </IconAIContainer>
            {'AI Assistant'}
        </RHSTitleContainer>
    );
};

export default class Plugin {
    postEventListener: PostEventListener = new PostEventListener();

    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: any, store: WebappStore) {
        setupRedux(registry, store);

        let rhs: any = null;
        if ((window as any).Components.CreatePost) {
            rhs = registry.registerRightHandSidebarComponent(RHS, RHSTitle);
            setOpenRHSAction(rhs.showRHSPlugin);

            registry.registerReducer((state = {}, action: any) => {
                switch (action.type) {
                case 'SET_AI_BOT_CHANNEL':
                    return {
                        ...state,
                        botChannelId: action.botChannelId,
                    };
                case 'SELECT_AI_POST':
                    return {
                        ...state,
                        selectedPostId: action.postId,
                    };
                default:
                    return state;
                }
            });
        }

        let currentUserId = store.getState().entities.users.currentUserId;
        if (currentUserId) {
            getAIDirectChannel(currentUserId).then((botChannelId) => {
                store.dispatch({type: 'SET_AI_BOT_CHANNEL', botChannelId} as any);
            });
        }

        store.subscribe(() => {
            const state = store.getState();
            if (state && state.entities.users.currentUserId !== currentUserId) {
                currentUserId = state.entities.users.currentUserId;
                if (currentUserId) {
                    getAIDirectChannel(currentUserId).then((botChannelId) => {
                        store.dispatch({type: 'SET_AI_BOT_CHANNEL', botChannelId} as any);
                    });
                } else {
                    store.dispatch({type: 'SET_AI_BOT_CHANNEL', botChannelId: ''} as any);
                }
            }
        });

        registry.registerWebSocketEventHandler(StreamingPostWebsocketEvent, this.postEventListener.handlePostUpdateWebsockets);
        const LLMBotPostWithWebsockets = (props: any) => {
            return (
                <LLMBotPost
                    {...props}
                    websocketRegister={this.postEventListener.registerPostUpdateListener}
                    websocketUnregister={this.postEventListener.unregisterPostUpdateListener}
                />
            )
            ;
        };

        registry.registerPostTypeComponent('custom_llmbot', LLMBotPostWithWebsockets);
        if (registry.registerPostActionComponent) {
            registry.registerPostActionComponent(PostMenu);
        } else {
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</>, (postId: string) => {
                const state = store.getState();
                const team = state.entities.teams.teams[state.entities.teams.currentTeamId];
                window.WebappUtils.browserHistory.push('/' + team.name + '/messages/@' + BotUsername);
                doSummarize(postId);
                if (rhs) {
                    store.dispatch(rhs.showRHSPlugin);
                }
            });
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconThreadSummarization/></span>{'Summarize Meeting Audio'}</>, doTranscribe);
            registry.registerPostDropdownMenuAction(<><span className='icon'><IconReactForMe/></span>{'React for me'}</>, doReaction);
        }
        if (registry.registerPostEditorActionComponent) {
            registry.registerPostEditorActionComponent(EditorMenu);
        }

        registry.registerAdminConsoleCustomSetting('Config', Config);
        if (rhs) {
            registry.registerChannelHeaderButtonAction(<IconAIContainer className='icon'><IconAI/></IconAIContainer>, () => {
                store.dispatch(rhs.toggleRHSPlugin);
            });
        }

        if (registry.registerCodeBlockActionComponent) {
            registry.registerCodeBlockActionComponent(CodeMenu);
        }
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void
        WebappUtils: any
    }
}

window.registerPlugin(manifest.id, new Plugin());
