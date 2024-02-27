import React from 'react';

import {Post} from '@mattermost/types/posts';

import {doReaction, doSummarize} from '../client';

import {useSelectPost} from '@/hooks';

import {useIsBasicsLicensed} from '@/license';

import IconAI from './assets/icon_ai';
import IconReactForMe from './assets/icon_react_for_me';
import DotMenu, {DropdownMenuItem} from './dot_menu';
import IconThreadSummarization from './assets/icon_thread_summarization';
import {Divider, DropdownInfoOnlyVisibleToYou} from './dropdown_info';

type Props = {
    post: Post,
}

const PostMenu = (props: Props) => {
    const selectPost = useSelectPost();
    const post = props.post;
    const isBasicsLicensed = useIsBasicsLicensed();

    const summarizePost = async (postId: string) => {
        const result = await doSummarize(postId);
        selectPost(result.postid, result.channelid);
    };

    if (!isBasicsLicensed) {
        return null;
    }

    return (
        <DotMenu
            icon={<IconAI/>}
            title='AI Actions'
        >
            <DropdownMenuItem onClick={() => summarizePost(post.id)}><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => doReaction(post.id)}><span className='icon'><IconReactForMe/></span>{'React for me'}</DropdownMenuItem>
            <Divider/>
            <DropdownInfoOnlyVisibleToYou/>
        </DotMenu>
    );
};

export default PostMenu;
