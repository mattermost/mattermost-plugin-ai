import React, {useEffect, useState} from 'react';
import styled from 'styled-components';

import {WebSocketMessage} from '@mattermost/client';

import {doFeedback, doRegenerate, doStopGenerating} from '@/client';

import PostText from './post_text';
import IconThumbsUp from './assets/icon_thumbs_up';
import IconThumbsDown from './assets/icon_thumbs_down';
import IconRegenerate from './assets/icon_regenerate';
import IconCancel from './assets/icon_cancel';

const PostBody = styled.div`
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

    const userFeedbackPositive = () => {
        doFeedback(props.post.id, true);
    };

    const userFeedbackNegative = () => {
        doFeedback(props.post.id, false);
    };

    const regnerate = () => {
        doRegenerate(props.post.id);
    };

    const stopGenerating = () => {
        doStopGenerating(props.post.id);
    };

    return (
        <PostBody>
            <PostText
                message={message}
                channelID={props.post.channel_id}
                showCursor={generating}
            />
            <ControlsBar>
                { generating ? (
                    <StopGeneratingButton
                        onClick={stopGenerating}
                    >
                        <IconCancel/>
                        {'Stop Generating'}
                    </StopGeneratingButton>
                ) : (
                    <GenerationButton
                        onClick={regnerate}
                    >
                        <IconRegenerate/>
                        {'Regenerate'}
                    </GenerationButton>
                )}
                {/*<RatingsContainer>
                    <EmojiButton
                        onClick={userFeedbackPositive}
                    >
                        <ThumbsUp/>
                    </EmojiButton>
                    <EmojiButton
                        onClick={userFeedbackNegative}
                    >
                        <ThumbsDown/>
                    </EmojiButton>
                </RatingsContainer>*/}
            </ControlsBar>
        </PostBody>
    );
};
