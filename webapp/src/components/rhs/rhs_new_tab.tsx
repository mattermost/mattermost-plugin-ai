import React, {useState, useCallback} from 'react';
import styled from 'styled-components';
import {useIntl, FormattedMessage} from 'react-intl';

import {
    FormatListNumberedIcon,
    LightbulbOutlineIcon,
    PlaylistCheckIcon,
} from '@mattermost/compass-icons/components';

import {useDispatch} from 'react-redux';

import RHSImage from '../assets/rhs_image';

import {createPost} from '@/client';

import {Button} from './common';

const CreatePost = (window as any).Components.CreatePost;

const CreatePostContainer = styled.div`
	.custom-textarea {
		padding-top: 13px;
		padding-bottom: 13px;
		passing-left: 16px;
	}
    .AdvancedTextEditor {
        padding: 0px;
    }
`;

const OptionButton = styled(Button)`
    color: rgb(var(--link-color-rgb));
    background-color: rgba(var(--button-bg-rgb), 0.08);
    svg {
        fill: rgb(var(--link-color-rgb));
    }
    &:hover {
        background-color: rgba(var(--button-bg-rgb), 0.12);
	}
	font-weight: 600;
	line-height: 16px;
	font-size: 12px;
`;

const NewQuestion = styled.div`
	margin: 0 24px;
	margin-top: 16px;
    display: flex;
    flex-direction: column;
	gap: 8px;
`;

const QuestionTitle = styled.div`
    font-family: Metropolis;
    font-weight: 600;
    font-size: 22px;
`;

const QuestionDescription = styled.div`
    font-weight: 400;
    font-size: 14px;
`;

const QuestionOptions = styled.div`
    display: flex;
	gap: 8px;
	margin-top: 16px;
	margin-bottom: 24px;
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

const RHSNewTab = ({botChannelId, selectPost, setCurrentTab}: Props) => {
    const dispatch = useDispatch();
    const intl = useIntl();
    const [draft, updateDraft] = useState<any>(null);
    const addBrainstormingIdeas = useCallback(() => {
        setEditorText(intl.formatMessage({id: 'rhs_new_tab.brainstorm_ideas_prompt', defaultMessage: 'Brainstorm ideas about '}));
    }, []);

    const addMeetingAgenda = useCallback(() => {
        setEditorText(intl.formatMessage({id: 'rhs_new_tab.meeting_agenda_prompt', defaultMessage: 'Write a meeting agenda about '}));
    }, []);

    const addToDoList = useCallback(() => {
        setEditorText(intl.formatMessage({id: 'rhs_new_tab.to_do_list_prompt', defaultMessage: 'Write a todo list about '}));
    }, []);

    const addProsAndCons = useCallback(() => {
        setEditorText(intl.formatMessage({id: 'rhs_new_tab.pros_and_cons_prompt', defaultMessage: 'Write a pros and cons list about '}));
    }, []);
    return (
        <NewQuestion>
            <RHSImage/>
            <QuestionTitle>
                <FormattedMessage
                    id='rhs_new_tab.ask_copilot_anything_title'
                    defaultMessage='Ask Copilot anything'
                />
            </QuestionTitle>
            <QuestionDescription>
                <FormattedMessage
                    id='rhs_new_tab.ask_copilot_anything_description'
                    defaultMessage='The Copilot is here to help. Choose from the prompts below or write your own.'
                />
            </QuestionDescription>
            <QuestionOptions>
                <OptionButton onClick={addBrainstormingIdeas}><LightbulbOutlineIcon/>
                    <FormattedMessage
                        id='rhs_new_tab.brainstorm_ideas'
                        defaultMessage='Brainstorm ideas'
                    />
                </OptionButton>
                <OptionButton onClick={addMeetingAgenda}><FormatListNumberedIcon/>
                    <FormattedMessage
                        id='rhs_new_tab.meeting_agenda'
                        defaultMessage='Meeting agenda'
                    />
                </OptionButton>
                <OptionButton onClick={addProsAndCons}><PlusMinus className='icon'>{'±'}</PlusMinus>
                    <FormattedMessage
                        id='rhs_new_tab.pros_and_cons'
                        defaultMessage='Pros and Cons'
                    />
                </OptionButton>
                <OptionButton onClick={addToDoList}><PlaylistCheckIcon/>
                    <FormattedMessage
                        id='rhs_new_tab.to_do_list'
                        defaultMessage='To-do list'
                    />
                </OptionButton>
            </QuestionOptions>
            <CreatePostContainer>
                <CreatePost
                    data-testid='rhs-new-tab-create-post'
                    channelId={botChannelId}
                    placeholder={intl.formatMessage({id: 'rhs_new_tab.ask_copilot_anything', defaultMessage: 'Ask Copilot anything...'})}
                    rootId={'ai_copilot'}
                    onSubmit={async (p: any) => {
                        const post = {...p};
                        post.channel_id = botChannelId || '';
                        post.props = {};
                        post.uploadsInProgress = [];
                        post.file_ids = p.fileInfos.map((f: any) => f.id);
                        const created = await createPost(post);
                        selectPost(created.id);
                        setCurrentTab('thread');
                        dispatch({
                            type: 'SET_GLOBAL_ITEM',
                            data: {
                                name: 'comment_draft_ai_copilot',
                                value: {message: '', fileInfos: [], uploadsInProgress: []},
                            },
                        });
                    }}
                    draft={draft}
                    onUpdateCommentDraft={(newDraft: any) => {
                        updateDraft(newDraft);
                        const timestamp = new Date().getTime();
                        newDraft.updateAt = timestamp;
                        newDraft.createAt = newDraft.createAt || timestamp;
                        dispatch({
                            type: 'SET_GLOBAL_ITEM',
                            data: {
                                name: 'comment_draft_ai_copilot',
                                value: newDraft,
                            },
                        });
                    }}
                />
            </CreatePostContainer>
        </NewQuestion>
    );
};

export default React.memo(RHSNewTab);
