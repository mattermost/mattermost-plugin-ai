import React from 'react';
import {useSelector} from 'react-redux';

import {GlobalState} from '@mattermost/types/store';
import {Team} from '@mattermost/types/teams';
import {Post} from '@mattermost/types/posts';

import IconAI from './assets/icon_ai';
import IconReactForMe from './assets/icon_react_for_me';
import DotMenu, {DropdownMenuItem} from './dot_menu';
import IconThreadSummarization from './assets/icon_thread_summarization';
import {doReaction, doTranscribe, doSummarize} from '../client';
import {BotUsername} from '../constants';

type Props = {
    post: Post,
}

const PostMenu = (props: Props) => {
    const post = props.post;
    const team = useSelector<GlobalState, Team>((state) => state.entities.teams.teams[state.entities.teams.currentTeamId]);

    const summarizePost = (teamName: string, postId: string) => {
        window.WebappUtils.browserHistory.push('/' + teamName + '/messages/@' + BotUsername);
        doSummarize(postId);
    };

    return (
        <DotMenu icon={<IconAI/>} title='AI Actions'>
            <DropdownMenuItem onClick={() => summarizePost(team.name, post.id)}><span className='icon'><IconThreadSummarization/></span>{'Summarize Thread'}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => doTranscribe(post.id)}><span className='icon'><IconThreadSummarization/></span>{'Summarize Meeting Audio'}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => doReaction(post.id)}><span className='icon'><IconReactForMe/></span>{'React for me'}</DropdownMenuItem>
        </DotMenu>
    );
};

export default PostMenu
