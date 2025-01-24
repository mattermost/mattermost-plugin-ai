// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

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

import {AdvancedTextEditor, CreatePost} from '@/mm_webapp';

import {Button, RHSPaddingContainer, RHSText, RHSTitle} from './common';

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

const ReverseScroll = styled.div`
	display: flex;
	flex-direction: column;
	flex-grow: 1;
	justify-content: flex-end;
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
    const intl = useIntl();

    // Compatibility with pre v10 create post export
    const dispatch = useDispatch();

    // Compatibility with pre v10 create post export
    const [draft, updateDraft] = useState<any>(null);

    const addBrainstormingIdeas = useCallback(() => {
        setEditorText(intl.formatMessage({defaultMessage: 'Brainstorm ideas about '}));
    }, []);

    const addMeetingAgenda = useCallback(() => {
        setEditorText(intl.formatMessage({defaultMessage: 'Write a meeting agenda about '}));
    }, []);

    const addToDoList = useCallback(() => {
        setEditorText(intl.formatMessage({defaultMessage: 'Write a todo list about '}));
    }, []);

    const addProsAndCons = useCallback(() => {
        setEditorText(intl.formatMessage({defaultMessage: 'Write a pros and cons list about '}));
    }, []);

    // Compatibility with pre v10 create post export
    let editorComponent;
    if (AdvancedTextEditor) {
        editorComponent = (
            <AdvancedTextEditor
                channelId={botChannelId}
                placeholder={intl.formatMessage({defaultMessage: 'Ask Copilot anything...'})}
                isThreadView={true}
                location={'RHS_COMMENT'}
                afterSubmit={(result: {created?: {id: string}}) => {
                    if (result.created?.id) {
                        selectPost(result.created?.id);
                        setCurrentTab('thread');
                    }
                }}
            />
        );
    } else {
        editorComponent = (
            <CreatePost
                channelId={botChannelId}
                placeholder={intl.formatMessage({defaultMessage: 'Ask Copilot anything...'})}
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
        );
    }

    return (
        <RHSPaddingContainer>
            <ReverseScroll>
                <RHSImage/>
                <RHSTitle><FormattedMessage defaultMessage='Ask Copilot anything'/></RHSTitle>
                <RHSText><FormattedMessage defaultMessage='The Copilot is here to help. Choose from the prompts below or write your own.'/></RHSText>
                <QuestionOptions>
                    <OptionButton onClick={addBrainstormingIdeas}>
                        <LightbulbOutlineIcon/>
                        <FormattedMessage defaultMessage='Brainstorm ideas'/>
                    </OptionButton>
                    <OptionButton onClick={addMeetingAgenda}>
                        <FormatListNumberedIcon/>
                        <FormattedMessage defaultMessage='Meeting agenda'/>
                    </OptionButton>
                    <OptionButton onClick={addProsAndCons}>
                        <PlusMinus className='icon'>{'±'}</PlusMinus>
                        <FormattedMessage defaultMessage='Pros and Cons'/>
                    </OptionButton>
                    <OptionButton onClick={addToDoList}>
                        <PlaylistCheckIcon/>
                        <FormattedMessage defaultMessage='To-do list'/>
                    </OptionButton>
                </QuestionOptions>
                <CreatePostContainer
                    data-testid='rhs-new-tab-create-post'
                >
                    {editorComponent}
                </CreatePostContainer>
            </ReverseScroll>
        </RHSPaddingContainer>
    );
};

export default React.memo(RHSNewTab);
