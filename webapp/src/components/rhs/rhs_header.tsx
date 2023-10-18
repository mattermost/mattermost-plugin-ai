import React from 'react';
import styled from 'styled-components';

import {Button} from './common';

const MenuButton = styled(Button)`
    display: flex;
    margin-bottom: 12px;
    width: auto;
    &.new-button {
        color: rgb(var(--link-color-rgb));
        &:hover {
            background: transparent;
            color: rgb(var(--link-color-rgb));
        }
    }
    &.no-clickable {
        &:hover {
            background: transparent;
            color: rgb(var(--center-channel-color));
            cursor: unset;
        }
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

const Header = styled.div`
    display: flex;
    padding: 12px 12px 0 12px;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    flex-wrap: wrap;
`;

type Props = {
    currentTab: string
    setCurrentTab: (tab: string) => void
    selectPost: (postId: string) => void
}

const RHSHeader = ({currentTab, setCurrentTab, selectPost}: Props) => {
    return (
        <Header>

            {currentTab === 'threads' && (
                <MenuButton className='no-clickable'>
                    <i className='icon icon-clock-outline'/> {'Chat history'}
                </MenuButton>
            )}

            {currentTab !== 'threads' && (
                <MenuButton
                    onClick={() => {
                        setCurrentTab('threads');
                        selectPost('');
                    }}
                >
                    <i className='icon icon-clock-outline'/> {'View chat history'}
                </MenuButton>)}

            <HeaderSpacer/>

            {currentTab !== 'new' && (
                <MenuButton
                    className='new-button'
                    onClick={() => {
                        setCurrentTab('new');
                        selectPost('');
                    }}
                >
                    <i className='icon icon-pencil-outline'/> {'New chat'}
                </MenuButton>
            )}
        </Header>
    );
};

export default React.memo(RHSHeader);
