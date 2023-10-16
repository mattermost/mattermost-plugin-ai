import React, {useState, useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';
import styled from 'styled-components';
import {
    FormatListBulletedIcon,
    FormatListNumberedIcon,
    LightbulbOutlineIcon,
    PlaylistCheckIcon,
} from '@mattermost/compass-icons/components';

import {makeGetPostsInChannel, getPost} from 'mattermost-redux/selectors/entities/posts';
import {getAllDirectChannels} from 'mattermost-redux/selectors/entities/channels';
import {getPosts, createPostImmediately} from 'mattermost-redux/actions/posts';

import {manifest} from '@/manifest';

import RHSImage from '../assets/rhs_image';
import IconThread from '../assets/icon_thread';
import ThreadItem from './thread_item';

const AdvancedCreatePost = styled((window as any).Components.AdvancedCreatePost)`
`;
const AdvancedCreateComment = styled((window as any).Components.AdvancedCreateComment)`
`;

const ThreadViewer = styled((window as any).Components.ThreadViewer)`
    height: 100%;
`;

const ThreadsList = styled.div`
    overflow-y: scroll;
`;

const RhsContainer = styled.div`
    height: 100%;
    display: flex;
    flex-direction: column;
`;

const Header = styled.div`
    display: flex;
    padding: 12px 12px 0 12px;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    flex-wrap: wrap;
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
    display: flex;
    margin-bottom: 12px;
    width: auto;
    svg {
        min-width: 24px;
    }
    .thread-title {
        display: inline-block;
        max-width: 220px;
        text-overflow: ellipsis;
        white-space: nowrap;
        overflow: hidden;
    }
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


type Props = {}

export default function RHS(props: Props) {
    const dispatch = useDispatch();
    const [currentTab, setCurrentTab] = useState('new');

    const channels = useSelector(getAllDirectChannels)
    const aiDM = channels.find((v) => v.display_name == 'ai');
    const getPostsInChannel = makeGetPostsInChannel()
    let posts = useSelector((state) => getPostsInChannel(state as any, aiDM?.id || '', -1)) || []
    posts = posts.sort((a, b) => b.update_at - a.update_at)

    const selectedPostId = useSelector((state: any) => state["plugins-" + manifest.id].selectedPostId);
    const selectedPost = useSelector((state: any) => getPost(state, selectedPostId));

    useEffect(() => {
        if (currentTab === 'threads') {
            dispatch(getPosts(aiDM?.id || '', 0, 60, false, true, true) as any);
        }
    }, [currentTab]);

    const selectPost = (postId: string) => {
        dispatch({type: 'SELECT_AI_POST', postId})
    };

    let content = null;
    if (selectedPostId) {
        content = <ThreadViewer
            selected={selectedPostId}
            rootPostId={selectedPostId}
            useRelativeTimestamp={false}
            isThreadView={false}
        />
    } else if(currentTab == "threads") {
        content = (
            <ThreadsList>
                {posts.map((p) => (
                    <ThreadItem
                        key={p.id}
                        postMessage={p.message}
                        postFirstReply={p.message.split('\n').slice(1).join('\n').slice(1, 300)}
                        repliesCount={p.reply_count}
                        lastActivityDate={p.update_at}
                        onClick={() => {
                            setCurrentTab('thread')
                            selectPost(p.id);
                        }}
                    />))}
            </ThreadsList>
        )
    } else if (currentTab == 'new') {
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
                <AdvancedCreateComment
                    getChannelView={() => {}}
                    onSubmit={async (p: any) => {
                        p.channel_id = aiDM?.id || '';
                        const data = await dispatch(createPostImmediately(p) as any)
                        selectPost(data.data.id)
                        setCurrentTab('thread');
                    }}
                    onUpdateCommentDraft={(...args) => console.log("UPDATE DRAFT", args)}
                />
            </NewQuestion>
        )
    }
    const header = (
        <Header>
            {!selectedPost && (
                <MenuButton
                    className={currentTab === 'new' ? 'active' : ''}
                    onClick={() => {
                        setCurrentTab('new');
                        selectPost('');
                    }}
                >
                    <IconThread/> New thread
                </MenuButton>
            )}

            {selectedPost && (
                <MenuButton className='active'>
                    <IconThread/> <span className='thread-title'>{selectedPost.message.split("\n")[0]}</span>
                </MenuButton>
            )}

            <MenuButton
                className={currentTab === 'threads' ? 'active' : ''}
                onClick={() => {
                    setCurrentTab('threads');
                    selectPost('');
                }}
            >
                <FormatListBulletedIcon/>All threads
            </MenuButton>

            <HeaderSpacer/>

            <AddButton onClick={() => {
                setCurrentTab('new');
                selectPost('');
            }}>+</AddButton>
        </Header>
    )
    return (
        <RhsContainer>
            {header}
            {content}
        </RhsContainer>
    )
}
