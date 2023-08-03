import React from 'react';
import styled from 'styled-components';

import {doFeedback} from '@/client';

import PostText from './post_text';
import IconThumbsUp from './assets/icon_thumbs_up';
import IconThumbsDown from './assets/icon_thumbs_down';

interface Props {
    post: any;
}

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

export const LLMBotPost = (props: Props) => {
    const userFeedbackPositive = () => {
        doFeedback(props.post.id, true);
    };

    const userFeedbackNegative = () => {
        doFeedback(props.post.id, false);
    };

    return (
        <PostBody>
            <PostText
                post={props.post}
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
