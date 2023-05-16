import React from 'react';

import PostText from './post_text';

interface Props {
    post: any;
}

export const LLMBotPost = (props: Props) => {
    return (
        <div>
            <PostText
                post={props.post}
            />
        </div>
    );
};
