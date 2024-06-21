import React, {MouseEvent, useEffect, useRef, useState} from 'react';
import {FormattedMessage} from 'react-intl';
import {useSelector} from 'react-redux';
import styled, {css, createGlobalStyle} from 'styled-components';

import {WebSocketMessage} from '@mattermost/client';
import {GlobalState} from '@mattermost/types/store';

import {SendIcon} from '@mattermost/compass-icons/components';

import {doPostbackSummary, doRegenerate, doStopGenerating} from '@/client';

import {useSelectNotAIPost, useSelectPost} from '@/hooks';

import PostText from './post_text';
import IconRegenerate from './assets/icon_regenerate';
import IconCancel from './assets/icon_cancel';

const PostMessagePreview = (window as any).Components.PostMessagePreview;

const FixPostHover = createGlobalStyle<{disableHover?: string}>`
	${(props) => props.disableHover && css`
	&&&& {
		[data-testid="post-menu-${props.disableHover}"] {
			display: none !important;
		}
		[data-testid="post-menu-${props.disableHover}"]:hover {
			display: none !important;
		}
	}`}
`;

const PostBody = styled.div<{disableHover?: boolean}>`
	${(props) => props.disableHover && css`
	::before {
		content: '';
		position: absolute;
		width: 110%;
		height: 110%;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
	}`}
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

const StopGeneratingButton = styled.button`
	display: flex;
	padding: 5px 12px;
	align-items: center;
	justify-content: center;
	gap: 6px;
	border-radius: 4px;
	border: 1px solid rgba(var(--center-channel-color,0.12));
	background: var(--center-channel-bg);

	box-shadow: 0px 4px 6px 0px rgba(0, 0, 0, 0.12);

	position: absolute;
	left: 50%;
	top: -5px;
	transform: translateX(-50%);

	color: var(--button-bg);

	font-size: 12px;
	font-weight: 600;
`;

const QuestionAnswerMark = styled.div`
    margin-top: 8px;
	display: inline-flex;
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

const Question = styled.div`
	font-size: 20px;
	font-style: normal;
	font-weight: 600;
	line-height: 28px;
`;

export interface PostUpdateWebsocketMessage {
    next: string
    post_id: string
    control?: string
}

interface Props {
    post: any;
    websocketRegister: (postID: string, handler: (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => void) => void;
    websocketUnregister: (postID: string) => void;
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

    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);
    const rootPost = useSelector<GlobalState, any>((state) => state.entities.posts.posts[props.post.root_id]);

    useEffect(() => {
        props.websocketRegister(props.post.id, (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
            const data = msg.data;
            if (!data.control && !stoppedRef.current) {
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
            props.websocketUnregister(props.post.id);
        };
    }, []);

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

    const stopPropagationIfGenerating = (e: MouseEvent) => {
        if (generating) {
            e.stopPropagation();
        }
    };

    const postSummary = async () => {
        const result = await doPostbackSummary(props.post.id);
        selectPost(result.rootid, result.channelid);
    };

    const isSearchResult = Boolean(props.post.props?.search_query);
    const requesterIsCurrentUser = (props.post.props?.llm_requester_user_id === currentUserId);
    const isThreadSummaryPost = (props.post.props?.referenced_thread && props.post.props?.referenced_thread !== '');
    const isNoShowRegen = (props.post.props?.no_regen && props.post.props?.no_regen !== '');
    const isTranscriptionResult = rootPost?.props?.referenced_transcript_post_id && rootPost?.props?.referenced_transcript_post_id !== '';

    let permalinkView = null;
    if (PostMessagePreview) { // Ignore permalink if version does not exporrt PostMessagePreview
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
    const showControlsBar = (showRegenerate || showPostbackButton) && message !== '';

    const searchResults = JSON.parse(props.post.props?.search_results);
    searchResults?.map((result: any) => {
        const {PostID, Message} = result;
        console.log(PostID, Message);
        return null;
    });

    return (
        <PostBody
            data-testid='llm-bot-post'
            disableHover={generating}
            onMouseOver={stopPropagationIfGenerating}
            onMouseEnter={stopPropagationIfGenerating}
            onMouseMove={stopPropagationIfGenerating}
        >
            {isSearchResult && (
                <QuestionAnswerMark>
                    <FormattedMessage defaultMessage='Question'/>
                </QuestionAnswerMark>
            )}
            {isSearchResult && (<Question>{props.post.props?.search_query}</Question>)}

            {isSearchResult && (
                <QuestionAnswerMark>
                    <FormattedMessage defaultMessage='Answer'/>
                </QuestionAnswerMark>
            )}

            <FixPostHover disableHover={generating ? props.post.id : ''}/>
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
            { generating && requesterIsCurrentUser &&
            <StopGeneratingButton
                data-testid='stop-generating-button'
                onClick={stopGenerating}
            >
                <IconCancel/>
                <FormattedMessage defaultMessage='Stop Generating'/>
            </StopGeneratingButton>
            }
            { showPostbackButton &&
            <PostSummaryHelpMessage>
                <FormattedMessage defaultMessage='Would you like to post this summary to the original call thread? You can also ask Copilot to make changes.'/>
            </PostSummaryHelpMessage>
            }
            { showControlsBar &&
            <ControlsBar>
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

