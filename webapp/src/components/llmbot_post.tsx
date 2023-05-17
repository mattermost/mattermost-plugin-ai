import React from 'react';
import styled from 'styled-components';

import {doFeedback} from '@/client';

import PostText from './post_text';

interface Props {
    post: any;
}

const PostBody = styled.div`
	font-family: monospace;
	word-spacing: -0.1rem;
`;

const FeedbackBar = styled.div`
	display: flex;
	flex-direction: row;
	justify-content: center;
	border-radius: 4px;
`;

const RatingsContainer = styled.div`
	display: flex;
	flex-direction: row;
	gap: 5px;
	border-radius: 4px;
	padding: 2px;
	background: rgba(var(--center-channel-color-rgb), 0.08);
`;

const EmojiButton = styled.div`
	width: 30px;
	height: 30px;
	padding: 5px;
	border-radius: 4px;

	:hover {
		background: rgba(var(--center-channel-color-rgb), 0.08);
	}

	:click {
		background: rgba(var(--center-channel-color-rgb), 0.20);
	}
`;

const EmojiIcon = styled.div`
	width: 20px;
	height: 20px;
	display: inline-flex;
	background-size: contain;
	background-position: 50% 50%;
	background-repeat: no-repeat;

`;

const PlusOne = styled(EmojiIcon)`
	background-image: url("/static/emoji/1f44d.png");
`;

const MinusOne = styled(EmojiIcon)`
	background-image: url("/static/emoji/1f44e.png");
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
            <FeedbackBar>
                <RatingsContainer>
                    <EmojiButton
                        onClick={userFeedbackPositive}
                    >
                        <PlusOne/>
                    </EmojiButton>
                    <EmojiButton
                        onClick={userFeedbackNegative}
                    >
                        <MinusOne/>
                    </EmojiButton>
                </RatingsContainer>
            </FeedbackBar>
        </PostBody>
    );
};
