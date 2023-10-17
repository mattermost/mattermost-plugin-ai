import React from 'react';
import {useDispatch} from 'react-redux';
import styled from 'styled-components';

import {
    FormatListNumberedIcon,
    LightbulbOutlineIcon,
    PlaylistCheckIcon,
} from '@mattermost/compass-icons/components';

import {createPostImmediately} from 'mattermost-redux/actions/posts';

import RHSImage from '../assets/rhs_image';

import {Button} from './common';

const AdvancedCreateComment = styled((window as any).Components.AdvancedCreateComment)`
    padding: 0px;
`;

const OptionButton = styled(Button)`
    color: rgb(var(--link-color-rgb));
    background-color: rgba(var(--button-bg-rgb), 0.04);
    svg {
        fill: rgb(var(--link-color-rgb));
    }
`;


const NewQuestion = styled.div`
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    flex-grow: 1;
    .AdvancedTextEditor {
        padding: 0px;
    }
`;

const QuestionTitle = styled.div`
    font-weight: 600;
    font-size: 22px;
`;

const QuestionDescription = styled.div`
    font-weight: 400;
    font-size: 14px;
`;

const QuestionOptions = styled.div`
    display: flex;
    flex-wrap: wrap;
`;

type Props = {
    botChannelId: string
    selectPost: (postId: string) => void
    setCurrentTab: (tab: string) => void
}

const RHSNewTab = ({botChannelId, selectPost, setCurrentTab}: Props) => {
    const dispatch = useDispatch();
    return (
        <NewQuestion>
            <RHSImage/>
            <QuestionTitle>{'Ask AI Assistant anything'}</QuestionTitle>
            <QuestionDescription>{'The AI Assistant can help you with almost anything. Choose from the prompts below or write your own.'}</QuestionDescription>
            <QuestionOptions>
                <OptionButton><LightbulbOutlineIcon/>{'Brainstorm ideas'}</OptionButton>
                <OptionButton><FormatListNumberedIcon/>{'Meeting agenda'}</OptionButton>
                <OptionButton><PlaylistCheckIcon/>{'To-do list'}</OptionButton>
                <OptionButton>{'Pros and Cons'}</OptionButton>
            </QuestionOptions>
            <AdvancedCreateComment
                rootId={undefined}
                rootDeleted={false}
                updateCommentDraftWithRootId={() => null}
                onMoveHistoryIndexBack={() => null}
                onMoveHistoryIndexForward={() => null}
                onEditLatestPost={() => null}
                getChannelView={() => null}
                onSubmit={async (p: any) => {
                    p.channel_id = botChannelId || '';
                    const data = await dispatch(createPostImmediately(p) as any);
                    selectPost(data.data.id);
                    setCurrentTab('thread');
                }}
                onUpdateCommentDraft={() => null}
            />
        </NewQuestion>
    );
}

export default React.memo(RHSNewTab)
