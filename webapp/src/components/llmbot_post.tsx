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

const RatingsContainer = styled.div`
	display: flex;
	flex-direction: row;
	justify-content: flex-end;
	height: 20px;
	gap: 5px;
`;

const EmojiButton = styled.div`
	display: flex;
	justify-content: center;
	align-content: center;
	width: 25px;
	height: 25px;
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
        </PostBody>
    );
};
