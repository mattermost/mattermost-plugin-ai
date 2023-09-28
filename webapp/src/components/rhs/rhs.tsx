import React, {useState, useEffect} from 'react';
import styled from 'styled-components';
import {
    FormatListBulletedIcon,
    FormatListNumberedIcon,
    LightbulbOutlineIcon,
    PlaylistCheckIcon,
} from '@mattermost/compass-icons/components';

import RHSImage from '../assets/rhs_image';
import IconThread from '../assets/icon_thread';
import ThreadItem from './thread_item';

const AdvancedCreatePost = styled((window as any).Components.AdvancedCreatePost)`
`;

const ThreadViewer = styled((window as any).Components.ThreadViewer)`
    height: 100%;
`;

const RhsContainer = styled.div`
    height: 100%;
    display: flex;
    flex-direction: column;
`;

const Header = styled.div`
    display: flex;
    padding: 12px;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12)
`;

const Button = styled.button`
    border-radius: 4px;
    padding: 8px 16px;
    display: flex;
    align-items: center;
    font-weight: 600;
    font-size: 12px;
    background-color: rgb(var(--center-channel-bg-rgb));
    color: rgba(var(--center-channel-color), 0.6);
    width: 172px;
    border: 0;
    margin: 0 8px 8px 0;

    &:hover {
        background-color: rgba(var(--button-bg-rgb), 0.08);
        color: rgb(var(--link-color-rgb));
        svg {
            fill: rgb(var(--link-color-rgb))
        }
    }

    &.active {
        color: rgb(var(--link-color-rgb));
        background-color: rgba(var(--button-bg-rgb), 0.04);
        svg {
            fill: rgb(var(--link-color-rgb))
        }
    }

    svg {
        fill: rgb(var(--center-channel-color));
        margin-right: 6px;
    }
`;

const OptionButton = styled(Button)`
    color: rgb(var(--link-color-rgb));
    background-color: rgba(var(--button-bg-rgb), 0.04);
    svg {
        fill: rgb(var(--link-color-rgb));
    }
`

const MenuButton = styled(Button)`
    margin-bottom: 0;
    width: auto;
`;

const HeaderSpacer = styled.div`
    flex-grow: 1;
`;

const AddButton = styled(Button)`
    border-radius: 50%;
    width: 28px;
    height: 28px;
    background-color: rgba(var(--center-channel-color-rgb), 0.04);
    display: flex;
    align-items: center;
    justify-content: center;
`;

const NewQuestion = styled.div`
    padding: 12px;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    flex-grow: 1;
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
    selectedPostId: string;
    threadsList: boolean;
}

export default function RHS(props: Props) {
    const [currentTab, setCurrentTab] = useState('new');
    useEffect(() => {
        setCurrentTab(props.selectedPostId ? 'thread' : 'new');
    }, []);

    let content = null;
    switch(currentTab) {
    case 'thread':
        content = <ThreadViewer
            rootPostId={props.selectedPostId}
            useRelativeTimestamp={false}
            isThreadView={false}
        />
                break;
    case 'threads':
        content = (
            <div>
                <ThreadItem
                    key={'key1'}
                    postMessage={'Title here with a long enough text to make the ellipsis'}
                    postFirstReply={'Some text to include in the body to make it cool and also have to have at least 2 lines and whenever it reaches the end of the second line it should add ellipsis'}
                    repliesCount={3}
                    lastActivityDate={5}
                    onClick={() => {
                        console.log("CLIKING");
                        setCurrentTab('thread')
                        // TODO: Change the selected post
                    }}
                />
                <ThreadItem
                    key={'key2'}
                    postMessage={'Title here with a long enough text to make the ellipsis'}
                    postFirstReply={'Some text to include in the body to make it cool and also have to have at least 2 lines and whenever it reaches the end of the second line it should add ellipsis'}
                    repliesCount={3}
                    lastActivityDate={5}
                    onClick={() => {
                        console.log("CLIKING");
                        setCurrentTab('thread')
                        // TODO: Change the selected post
                    }}
                />
            </div>
        )
        break
    case 'new':
        content = (
            <NewQuestion>
                <RHSImage />
                <QuestionTitle>{'Ask AI Assistant anything'}</QuestionTitle>
                <QuestionDescription>{'The AI Assistant can help you with almost anything. Choose from the prompts below or write your own.'}</QuestionDescription>
                <QuestionOptions>
                    <OptionButton><LightbulbOutlineIcon/>{'Brainstorm ideas'}</OptionButton>
                    <OptionButton><FormatListNumberedIcon/>{'Meeting agenda'}</OptionButton>
                    <OptionButton><PlaylistCheckIcon/>{'To-do list'}</OptionButton>
                    <OptionButton>{'Pros and Cons'}</OptionButton>
                </QuestionOptions>
                <AdvancedCreatePost getChannelView={() => {}}/>
            </NewQuestion>
        )
        break;
    }
    const header = (
        <Header>
            <MenuButton
                className={currentTab === 'new' ? 'active' : ''}
                onClick={() => setCurrentTab('new')}
            >
                <IconThread/> New thread
            </MenuButton>

            <MenuButton
                className={currentTab === 'threads' ? 'active' : ''}
                onClick={() => setCurrentTab('threads')}
            >
                <FormatListBulletedIcon/>All threads
            </MenuButton>

            <HeaderSpacer/>

            <AddButton onClick={() => setCurrentTab('new')}>+</AddButton>
        </Header>
    )
    return (
        <RhsContainer>
            {header}
            {content}
        </RhsContainer>
    )
}
