// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {FormattedMessage} from 'react-intl';
import {useSelector} from 'react-redux';
import styled from 'styled-components';

import {WebSocketMessage} from '@mattermost/client';
import {GlobalState} from '@mattermost/types/store';

import {SendIcon} from '@mattermost/compass-icons/components';

import {doPostbackSummary, doRegenerate, doStopGenerating} from '@/client';

import {useSelectNotAIPost} from '@/hooks';

import {PostMessagePreview} from '@/mm_webapp';

import {SearchSources} from './search_sources';

import PostText from './post_text';
import IconRegenerate from './assets/icon_regenerate';
import IconCancel from './assets/icon_cancel';
import ToolApprovalSet from './tool_approval_set';

const SearchResultsPropKey = 'search_results';

const PostBody = styled.div`
`;

const ControlsBar = styled.div`
	display: flex;
	flex-direction: row;
	justify-content: left;
	height: 28px;
	margin-top: 8px;
	gap: 4px;
`;

const GenerationButton = styled.button`
	display: flex;
	border: none;
	height: 24px;
	padding: 4px 10px;
	align-items: center;
	justify-content: center;
	gap: 6px;
	border-radius: 4px;
	background: rgba(var(--center-channel-color-rgb), 0.08);
    color: rgba(var(--center-channel-color-rgb), 0.64);

	font-size: 12px;
	line-height: 16px;
	font-weight: 600;

	:hover {
		background: rgba(var(--center-channel-color-rgb), 0.12);
        color: rgba(var(--center-channel-color-rgb), 0.72);
	}

	:active {
		background: rgba(var(--button-bg-rgb), 0.08);
	}
`;

const PostSummaryButton = styled(GenerationButton)`
	background: var(--button-bg);
    color: var(--button-color);

	:hover {
		background: rgba(var(--button-bg-rgb), 0.88);
		color: var(--button-color);
	}

	:active {
		background: rgba(var(--button-bg-rgb), 0.92);
	}
`;

const StopGeneratingButton = styled(GenerationButton)`
`;

const PostSummaryHelpMessage = styled.div`
	font-size: 14px;
	font-style: italic;
	font-weight: 400;
	line-height: 20px;
	border-top: 1px solid rgba(var(--center-channel-color-rgb), 0.12);

	padding-top: 8px;
	padding-bottom: 8px;
	margin-top: 16px;
`;

export interface PostUpdateWebsocketMessage {
    post_id: string
    next?: string
    control?: string
    tool_call?: string
}

export enum ToolCallStatus {
    Pending = 0,
    Accepted = 1,
    Rejected = 2,
    Error = 3,
    Success = 4
}

export interface ToolCall {
    id: string;
    name: string;
    description: string;
    arguments: any;
    result?: string;
    status: ToolCallStatus;
}

interface Props {
    post: any;
    websocketRegister?: (postID: string, listenerID: string, handler: (msg: WebSocketMessage<any>) => void) => void;
    websocketUnregister?: (postID: string, listenerID: string) => void;
}

export const LLMBotPost = (props: Props) => {
    const selectPost = useSelectNotAIPost();
    const [message, setMessage] = useState(props.post.message);

    // Generating is true while we are reciving new content from the websocket
    const [generating, setGenerating] = useState(false);

    // Stopped is a flag that is used to prevent the websocket from updating the message after the user has stopped the generation
    // Needs a ref because of the useEffect closure.
    const [stopped, setStopped] = useState(false);
    const stoppedRef = useRef(stopped);
    stoppedRef.current = stopped;

    // State for tool calls
    const [toolCalls, setToolCalls] = useState<ToolCall[]>([]);
    const [error, setError] = useState('');

    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);
    const rootPost = useSelector<GlobalState, any>((state) => state.entities.posts.posts[props.post.root_id]);

    // Get tool calls from post props
    const toolCallsJson = props.post.props?.pending_tool_call;

    // Update tool calls from props when available
    useEffect(() => {
        if (toolCallsJson) {
            try {
                const parsedToolCalls = JSON.parse(toolCallsJson);
                setToolCalls(parsedToolCalls);
            } catch (error) {
                // Log error for debugging
                setError('Error parsing tool calls');
            }
        }
    }, [toolCallsJson]);

    // Use original websocket registration for listening
    useEffect(() => {
        if (props.websocketRegister && props.websocketUnregister) {
            const listenerID = Math.random().toString(36).substring(7);

            props.websocketRegister(props.post.id, listenerID, (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
                const data = msg.data;

                // Handle tool call events from the websocket event
                if (data.control === 'tool_call' && data.post_id === props.post.id && data.tool_call) {
                    try {
                        const parsedToolCalls = JSON.parse(data.tool_call);
                        setToolCalls(parsedToolCalls);
                    } catch (error) {
                        // Handle error silently
                        setError('Error parsing tool call data');
                    }
                    return;
                }

                // Handle regular post updates
                if (data.next && !stoppedRef.current) {
                    setGenerating(true);
                    setMessage(data.next);
                } else if (data.control === 'end') {
                    setGenerating(false);
                    setStopped(false);
                } else if (data.control === 'start') {
                    setGenerating(true);
                    setStopped(false);
                }
            });

            return () => {
                if (props.websocketUnregister) {
                    props.websocketUnregister(props.post.id, listenerID);
                }
            };
        }

        return () => {/* no cleanup */};
    }, [props.post.id]);

    const regnerate = () => {
        setGenerating(true);
        setStopped(false);
        setMessage('');
        doRegenerate(props.post.id);
    };

    const stopGenerating = () => {
        setStopped(true);
        setGenerating(false);
        doStopGenerating(props.post.id);
    };

    const postSummary = async () => {
        const result = await doPostbackSummary(props.post.id);
        selectPost(result.rootid, result.channelid);
    };

    const requesterIsCurrentUser = (props.post.props?.llm_requester_user_id === currentUserId);
    const isThreadSummaryPost = (props.post.props?.referenced_thread && props.post.props?.referenced_thread !== '');
    const isNoShowRegen = (props.post.props?.no_regen && props.post.props?.no_regen !== '');
    const isTranscriptionResult = rootPost?.props?.referenced_transcript_post_id && rootPost?.props?.referenced_transcript_post_id !== '';

    let permalinkView = null;
    if (PostMessagePreview) { // Ignore permalink if version does not export PostMessagePreview
        const permalinkData = extractPermalinkData(props.post);
        if (permalinkData !== null) {
            permalinkView = (
                <PostMessagePreview
                    data-testid='llm-bot-permalink'
                    metadata={permalinkData}
                />
            );
        }
    }

    const showRegenerate = !generating && requesterIsCurrentUser && !isNoShowRegen;
    const showPostbackButton = !generating && requesterIsCurrentUser && isTranscriptionResult;
    const showStopGeneratingButton = generating && requesterIsCurrentUser;
    const showControlsBar = (showRegenerate || showPostbackButton || showStopGeneratingButton) && message !== '';

    return (
        <PostBody
            data-testid='llm-bot-post'
        >
            {error && <div className='error'>{error}</div>}
            {isThreadSummaryPost && permalinkView &&
            <>
                {permalinkView}
            </>
            }
            <PostText
                message={message}
                channelID={props.post.channel_id}
                postID={props.post.id}
                showCursor={generating}
            />
            {props.post.props?.[SearchResultsPropKey] && (
                <SearchSources
                    sources={JSON.parse(props.post.props[SearchResultsPropKey])}
                />
            )}
            {toolCalls && toolCalls.length > 0 && (
                <ToolApprovalSet
                    postID={props.post.id}
                    toolCalls={toolCalls}
                />
            )}
            { showPostbackButton &&
            <PostSummaryHelpMessage>
                <FormattedMessage defaultMessage='Would you like to post this summary to the original call thread? You can also ask Copilot to make changes.'/>
            </PostSummaryHelpMessage>
            }
            { showControlsBar &&
            <ControlsBar>
                { showStopGeneratingButton &&
                <StopGeneratingButton
                    data-testid='stop-generating-button'
                    onClick={stopGenerating}
                >
                    <IconCancel/>
                    <FormattedMessage defaultMessage='Stop Generating'/>
                </StopGeneratingButton>
                }
                {showPostbackButton &&
                <PostSummaryButton
                    data-testid='llm-bot-post-summary'
                    onClick={postSummary}
                >
                    <SendIcon/>
                    <FormattedMessage defaultMessage='Post summary'/>
                </PostSummaryButton>
                }
                { showRegenerate &&
                <GenerationButton
                    data-testid='regenerate-button'
                    onClick={regnerate}
                >
                    <IconRegenerate/>
                    <FormattedMessage defaultMessage='Regenerate'/>
                </GenerationButton>
                }
            </ControlsBar>
            }
        </PostBody>
    );
};

type PermalinkData = {
    channel_display_name: string
    channel_id: string
    post_id: string
    team_name: string
    post: {
        message: string
        user_id: string
    }
}

function extractPermalinkData(post: any): PermalinkData | null {
    for (const embed of post?.metadata?.embeds || []) {
        if (embed.type === 'permalink') {
            return embed.data;
        }
    }
    return null;
}
