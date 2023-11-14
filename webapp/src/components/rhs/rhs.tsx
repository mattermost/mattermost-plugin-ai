import React, {useState, useEffect, useCallback} from 'react';
import {useDispatch, useSelector} from 'react-redux';
import styled from 'styled-components';

import {makeGetPostsInChannel} from 'mattermost-redux/selectors/entities/posts';
import {getPosts} from 'mattermost-redux/actions/posts';

import {manifest} from '@/manifest';

import ThreadItem from './thread_item';
import RHSHeader from './rhs_header';
import RHSNewTab from './rhs_new_tab';

const ThreadViewer = (window as any).Components.ThreadViewer && styled((window as any).Components.ThreadViewer)`
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

export default function RHS() {
    const dispatch = useDispatch();
    const [currentTab, setCurrentTab] = useState('new');

    const botChannelId = useSelector((state: any) => state['plugins-' + manifest.id].botChannelId);
    const getPostsInChannel = makeGetPostsInChannel();
    let posts = useSelector((state) => getPostsInChannel(state as any, botChannelId || '', -1)) || [];
    posts = posts.filter((p) => !p.root_id).sort((a, b) => b.update_at - a.update_at);

    const selectedPostId = useSelector((state: any) => state['plugins-' + manifest.id].selectedPostId);

    useEffect(() => {
        if (currentTab === 'threads' && botChannelId ) {
            dispatch(getPosts(botChannelId, 0, 60, false, true, true) as any);
        }
    }, [currentTab, botChannelId]);

    const selectPost = useCallback((postId: string) => {
        dispatch({type: 'SELECT_AI_POST', postId});
    }, [dispatch]);

    let content = null;
    if (selectedPostId) {
        if (currentTab !== 'thread') {
            setCurrentTab('thread');
        }
        content = (
            <ThreadViewer
                inputPlaceholder='Reply...'
                selected={selectedPostId}
                rootPostId={selectedPostId}
                useRelativeTimestamp={false}
                isThreadView={false}
            />
        );
    } else if (currentTab === 'threads') {
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
                            setCurrentTab('thread');
                            selectPost(p.id);
                        }}
                    />))}
            </ThreadsList>
        );
    } else if (currentTab === 'new') {
        content = (
            <RHSNewTab
                botChannelId={botChannelId}
                setCurrentTab={setCurrentTab}
                selectPost={selectPost}
            />
        );
    }
    return (
        <RhsContainer>
            <RHSHeader
                currentTab={currentTab}
                setCurrentTab={setCurrentTab}
                selectPost={selectPost}
            />
            {content}
        </RhsContainer>
    );
}
