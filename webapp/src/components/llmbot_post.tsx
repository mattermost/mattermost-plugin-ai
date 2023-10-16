import React, {useEffect, useState} from 'react';
import styled from 'styled-components';

import {WebSocketMessage} from '@mattermost/client';

import {doFeedback} from '@/client';

import PostText from './post_text';
import IconThumbsUp from './assets/icon_thumbs_up';
import IconThumbsDown from './assets/icon_thumbs_down';

const PostBody = styled.div`
`;

const ControlsBar = styled.div`
	display: flex;
	flex-direction: row;
	justify-content: flex-end;
	height: 28px;
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

export interface PostUpdateWebsocketMessage {
    next: string
    post_id: string
}

interface Props {
    post: any;
    websocketRegister: (postID: string, handler: (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => void) => void;
    websocketUnregister: (postID: string) => void;
}

export const LLMBotPost = (props: Props) => {
    const [message, setMessage] = useState(props.post.message);
    useEffect(() => {
        props.websocketRegister(props.post.id, (msg: WebSocketMessage<PostUpdateWebsocketMessage>) => {
            const data = msg.data;
            setMessage(data.next);
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

    return (
        <PostBody>
            <PostText
                message={message}
                channelID={props.post.channel_id}
            />
            <ControlsBar>
                <RatingsContainer>
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
                </RatingsContainer>
            </ControlsBar>
        </PostBody>
    );
};
