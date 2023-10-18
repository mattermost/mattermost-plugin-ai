import React from 'react';
import {useDispatch} from 'react-redux';

import {Post} from '@mattermost/types/posts';

import {doReaction, doTranscribe, doSummarize, viewMyChannel} from '../client';

import {selectPost, openRHS} from '../redux_actions';

import IconAI from './assets/icon_ai';
import IconReactForMe from './assets/icon_react_for_me';
import DotMenu, {DropdownMenuItem} from './dot_menu';
import IconThreadSummarization from './assets/icon_thread_summarization';

type Props = {
    post: Post,
}

const selectPostLegacy = (postid: string, channelid: string) => {
    return {
        type: 'SELECT_POST',
        postId: postid,
        channelId: channelid,
        timestamp: Date.now(),
    };
};

const PostMenu = (props: Props) => {
    const dispatch = useDispatch();
    const post = props.post;

    const summarizePost = async (postId: string) => {
        const result = await doSummarize(postId);

        // This if is for legacy mode where the AdvancedCreatecomment is not exported
        if ((window as any).Components.CreatePost) {
            dispatch(selectPost(result.postid));
            dispatch(openRHS());
        } else {
            dispatch(selectPostLegacy(result.postid, result.channelid));
            viewMyChannel(result.channelid);
        }
    };

    return (
        <DotMenu
            icon={<IconAI/>}
            title='AI Actions'
        >
            <DropdownMenuItem onClick={() => summarizePost(post.id)}><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => doTranscribe(post.id)}><span className='icon'><IconThreadSummarization/></span>{'Summarize Meeting Audio'}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => doReaction(post.id)}><span className='icon'><IconReactForMe/></span>{'React for me'}</DropdownMenuItem>
        </DotMenu>
    );
};

export default PostMenu;
