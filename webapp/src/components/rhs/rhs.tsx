import React, {useState, useEffect, useCallback} from 'react';
import {useDispatch, useSelector} from 'react-redux';
import styled from 'styled-components';

import {manifest} from '@/manifest';

import {getAIThreads} from '@/client';

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

export interface AIThread {
    ID: string;
    Message: string;
    Title: string;
    ReplyCount: number;
    UpdateAt: number;
}

export default function RHS() {
    const dispatch = useDispatch();
    const [currentTab, setCurrentTab] = useState('new');
    const botChannelId = useSelector((state: any) => state['plugins-' + manifest.id].botChannelId);
    const selectedPostId = useSelector((state: any) => state['plugins-' + manifest.id].selectedPostId);

    const [threads, setThreads] = useState<AIThread[] | null>(null);

    useEffect(() => {
        const fetchThreads = async () => {
            setThreads(await getAIThreads());
        };
        if (currentTab === 'threads') {
            fetchThreads();
        }
    }, [currentTab]);

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
                rootPostId={selectedPostId}
                useRelativeTimestamp={false}
                isThreadView={false}
            />
        );
    } else if (currentTab === 'threads') {
        if (threads) {
            content = (
                <ThreadsList>
                    {threads.map((p) => (
                        <ThreadItem
                            key={p.ID}
                            postTitle={p.Title}
                            postMessage={p.Message}
                            repliesCount={p.ReplyCount}
                            lastActivityDate={p.UpdateAt}
                            onClick={() => {
                                setCurrentTab('thread');
                                selectPost(p.ID);
                            }}
                        />))}
                </ThreadsList>
            );
        } else {
            content = null;
        }
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
