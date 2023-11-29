import {useDispatch} from 'react-redux';

import {selectPost, openRHS} from 'src/redux_actions';

import {viewMyChannel} from 'src/client';

export const useSelectPost = () => {
    const dispatch = useDispatch();

    const selectPostLegacy = (postid: string, channelid: string) => {
        return {
            type: 'SELECT_POST',
            postId: postid,
            channelId: channelid,
            timestamp: Date.now(),
        };
    };

    return (postId: string, channelId: string) => {
        // This if is for legacy mode where the AdvancedCreatecomment is not exported
        if ((window as any).Components.CreatePost) {
            dispatch(selectPost(postId));
            dispatch(openRHS());
        } else {
            dispatch(selectPostLegacy(postId, channelId));
            viewMyChannel(channelId);
        }
    };
};

