import React, {MouseEvent, useEffect, useState} from 'react';
import {useSelector} from 'react-redux';
import styled, {css, createGlobalStyle} from 'styled-components';

import {WebSocketMessage} from '@mattermost/client';
import {GlobalState} from '@mattermost/types/store';

import {doFeedback, doRegenerate, doStopGenerating} from '@/client';

import PostText from './post_text';
import IconThumbsUp from './assets/icon_thumbs_up';
import IconThumbsDown from './assets/icon_thumbs_down';
import IconRegenerate from './assets/icon_regenerate';
import IconCancel from './assets/icon_cancel';

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
	justify-content: space-between;
	height: 28px;
	margin-top: 8px;
`;

const RatingsContainer = styled.div`
	display: flex;
	flex-direction: row;
	gap: 4px;
`;

const EmojiButton = styled.button`
	display: flex;
	align-items: center;
	justify-content: center;

	width: 24px;
	height: 24px;
	padding: 6px;
	border-radius: 4px;
	border: none;

	background: rgba(var(--center-channel-color-rgb), 0.04);

	:hover {
		background: rgba(var(--center-channel-color-rgb), 0.08);
        color: rgba(var(--center-channel-color-rgb), 0.72);
	}

	:active {
		background: rgba(var(--button-bg-rgb), 0.08);
	}
`;

const ThumbsUp = styled(IconThumbsUp)`
	width: 20px;
	height: 20px;
`;

const ThumbsDown = styled(IconThumbsDown)`
	width: 20px;
	height: 20px;
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
	background: rgba(var(--center-channel-color-rgb), 0.04);

	:hover {
		background: rgba(var(--center-channel-color-rgb), 0.08);
        color: rgba(var(--center-channel-color-rgb), 0.72);
	}

	:active {
		background: rgba(var(--button-bg-rgb), 0.08);
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
    const [message, setMessage] = useState(props.post.message);
    const [generating, setGenerating] = useState(false);
    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);
    useEffect(() => {
        props.websocketRegister(props.post.id, (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
            const data = msg.data;
            if (!data.control) {
                setGenerating(true);
                setMessage(data.next);
            } else if (data.control === 'end') {
                setGenerating(false);
            }
        });
        return () => {
            props.websocketUnregister(props.post.id);
        };
    }, []);

    const regnerate = () => {
        doRegenerate(props.post.id);
    };

    const stopGenerating = () => {
        doStopGenerating(props.post.id);
    };

    const stopPropagationIfGenerating = (e: MouseEvent) => {
        if (generating) {
            e.stopPropagation();
        }
    };

    const requesterIsCurrentUser = (props.post.props?.llm_requester_user_id === currentUserId);

    return (
        <PostBody
            disableHover={generating}
            onMouseOver={stopPropagationIfGenerating}
            onMouseEnter={stopPropagationIfGenerating}
            onMouseMove={stopPropagationIfGenerating}
        >
            <FixPostHover disableHover={generating ? props.post.id : ''}/>
            <PostText
                message={message}
                channelID={props.post.channel_id}
                showCursor={generating}
            />
            { generating && requesterIsCurrentUser &&
            <StopGeneratingButton
                onClick={stopGenerating}
            >
                <IconCancel/>
                {'Stop Generating'}
            </StopGeneratingButton>
            }
            { !generating && requesterIsCurrentUser &&
            <ControlsBar>
                <GenerationButton
                    onClick={regnerate}
                >
                    <IconRegenerate/>
                    {'Regenerate'}
                </GenerationButton>
            </ControlsBar>
            }
        </PostBody>
    );
};
