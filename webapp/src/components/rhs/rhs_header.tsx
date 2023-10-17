import React from 'react';
import styled from 'styled-components';
import {FormatListBulletedIcon} from '@mattermost/compass-icons/components';
import {Post} from 'mattermost-redux/types/posts';

import {Button} from './common';

import IconThread from '../assets/icon_thread';

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

const Header = styled.div`
    display: flex;
    padding: 12px 12px 0 12px;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    flex-wrap: wrap;
`;

type Props = {
    currentTab: string
    selectedPost: Post
    setCurrentTab: (tab: string) => void
    selectPost: (postId: string) => void
}

const RHSHeader = ({currentTab, selectedPost, setCurrentTab, selectPost}: Props) => {
    return (
        <Header>
            {!selectedPost && (
                <MenuButton
                    className={currentTab === 'new' ? 'active' : ''}
                    onClick={() => {
                        setCurrentTab('new');
                        selectPost('');
                    }}
                >
                    <IconThread/> {'New thread'}
                </MenuButton>
            )}

            {selectedPost && (
                <MenuButton className='active'>
                    <IconThread/> <span className='thread-title'>{selectedPost.message.split('\n')[0]}</span>
                </MenuButton>
            )}

            <MenuButton
                className={currentTab === 'threads' ? 'active' : ''}
                onClick={() => {
                    setCurrentTab('threads');
                    selectPost('');
                }}
            >
                <FormatListBulletedIcon/> {'All threads'}
            </MenuButton>

            <HeaderSpacer/>

            <AddButton
                onClick={() => {
                    setCurrentTab('new');
                    selectPost('');
                }}
            >{'+'}</AddButton>
        </Header>
    );
}

export default React.memo(RHSHeader);
