import React from 'react';

import {Post} from '@mattermost/types/posts';

import styled from 'styled-components';

import {doReaction, doSummarize} from '../client';

import {useSelectPost} from '@/hooks';

import {useIsBasicsLicensed} from '@/license';

import {useBotlist} from '@/bots';

import IconAI from './assets/icon_ai';
import IconReactForMe from './assets/icon_react_for_me';
import DotMenu, {DropdownMenu, DropdownMenuItem} from './dot_menu';
import IconThreadSummarization from './assets/icon_thread_summarization';
import {Divider, DropdownInfoOnlyVisibleToYou} from './dropdown_info';
import {DropdownBotSelector} from './bot_slector';

type Props = {
    post: Post,
}

const PostMenu = (props: Props) => {
    const selectPost = useSelectPost();
    const {bots, activeBot, setActiveBot} = useBotlist();
    const post = props.post;
    const isBasicsLicensed = useIsBasicsLicensed();

    const summarizePost = async (postId: string) => {
        const result = await doSummarize(postId, activeBot?.username || '');
        selectPost(result.postid, result.channelid);
    };

    if (!isBasicsLicensed) {
        return null;
    }

    // Unconfigured state
    if (bots && bots.length === 0) {
        return null;
    }

    return (
        <DotMenu
            icon={<IconAI/>}
            title='AI Actions'
            dropdownMenu={StyledDropdownMenu}
        >
            <DropdownBotSelector
                bots={bots ?? []}
                activeBot={activeBot}
                setActiveBot={setActiveBot}
            />
            <Divider/>
            <DropdownMenuItem onClick={() => summarizePost(post.id)}><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => doReaction(post.id)}><span className='icon'><IconReactForMe/></span>{'React for me'}</DropdownMenuItem>
            <Divider/>
            <DropdownInfoOnlyVisibleToYou/>
        </DotMenu>
    );
};

const StyledDropdownMenu = styled(DropdownMenu)`
	min-width: 240px;
`;

export default PostMenu;
