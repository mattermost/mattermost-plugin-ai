import {useDispatch} from 'react-redux';

import {selectPost, openRHS} from 'src/redux_actions';

import {viewMyChannel} from 'src/client';

const selectPostLegacy = (postid: string, channelid: string) => {
    return {
        type: 'SELECT_POST',
        postId: postid,
        channelId: channelid,
        timestamp: Date.now(),
    };
};

export const doSelectPost = (postId: string, channelId: string, dispatch: any) => {
    // This if is for legacy mode where the AdvancedCreatecomment is not exported
    if ((window as any).Components.CreatePost) {
        dispatch(selectPost(postId));
        dispatch(openRHS());
    } else {
        dispatch(selectPostLegacy(postId, channelId));
    }
    viewMyChannel(channelId);
};

export const useSelectPost = () => {
    const dispatch = useDispatch();

    return (postid: string, channelid: string) => {
        doSelectPost(postid, channelid, dispatch);
    };
};

