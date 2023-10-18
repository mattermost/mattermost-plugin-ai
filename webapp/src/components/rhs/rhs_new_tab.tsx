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

const CreatePost = styled((window as any).Components.CreatePost)`
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

const PlusMinus = styled.i`
    width: 14px;
    font-size: 14px;
    font-weight: 400;
    margin-right: 4px;
`;

type Props = {
    botChannelId: string
    selectPost: (postId: string) => void
    setCurrentTab: (tab: string) => void
}

const setEditorText = (text: string) => {
    const replyBox = document.getElementById('reply_textbox');
    if (replyBox) {
        replyBox.innerHTML = text;
        replyBox.dispatchEvent(new Event('input', {bubbles: true}));
        replyBox.focus();
    }
};

const addBrainstormingIdeas = () => {
    setEditorText('Brainstorm ideas about ');
};

const addMeetingAgenda = () => {
    setEditorText('Write a meeting agenda about ');
};

const addToDoList = () => {
    setEditorText('Write a todo list about ');
};

const addProsAndCons = () => {
    setEditorText('Write a pros and cons list about ');
};

const RHSNewTab = ({botChannelId, selectPost, setCurrentTab}: Props) => {
    const dispatch = useDispatch();
    return (
        <NewQuestion>
            <RHSImage/>
            <QuestionTitle>{'Ask AI Assistant anything'}</QuestionTitle>
            <QuestionDescription>{'The AI Assistant can help you with almost anything. Choose from the prompts below or write your own.'}</QuestionDescription>
            <QuestionOptions>
                <OptionButton onClick={addBrainstormingIdeas}><LightbulbOutlineIcon/>{'Brainstorm ideas'}</OptionButton>
                <OptionButton onClick={addMeetingAgenda}><FormatListNumberedIcon/>{'Meeting agenda'}</OptionButton>
                <OptionButton onClick={addToDoList}><PlaylistCheckIcon/>{'To-do list'}</OptionButton>
                <OptionButton onClick={addProsAndCons}><PlusMinus className='icon'>{'±'}</PlusMinus>{'Pros and Cons'}</OptionButton>
            </QuestionOptions>
            <CreatePost
                placeholder={'Ask AI Assistant anything...'}
                onSubmit={async (p: any) => {
                    p.channel_id = botChannelId || '';
                    p.props = {};
                    const data = await dispatch(createPostImmediately(p) as any);
                    selectPost(data.data.id);
                    setCurrentTab('thread');
                }}
            />
        </NewQuestion>
    );
};

export default React.memo(RHSNewTab);
