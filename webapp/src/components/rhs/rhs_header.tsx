import React from 'react';
import styled from 'styled-components';

import {Button} from './common';

const HistoryButton = styled(Button)`
    color: rgba(var(--center-channel-color-rgb), 0.64);
`;

const ButtonDisabled = styled(Button)`
	&:hover {
		background: transparent;
		color: rgb(var(--center-channel-color));
		cursor: unset;
	}
`;

const NewChatButton = styled(Button)`
	color: rgb(var(--link-color-rgb));
	&:hover {
		background: transparent;
		color: rgb(var(--link-color-rgb));
	}
`;

const Header = styled.div`
    display: flex;
	padding 8px;
	justify-content: space-between;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    flex-wrap: wrap;
`;

type Props = {
    currentTab: string
    setCurrentTab: (tab: string) => void
    selectPost: (postId: string) => void
}

const RHSHeader = ({currentTab, setCurrentTab, selectPost}: Props) => {
    let historyButton = null;
    if (currentTab === 'threads') {
        historyButton = (
            <ButtonDisabled>
                <i className='icon-clock-outline'/> {'Chat history'}
            </ButtonDisabled>
        );
    } else {
        historyButton = (
            <HistoryButton
                onClick={() => {
                    setCurrentTab('threads');
                    selectPost('');
                }}
            >
                <i className='icon-clock-outline'/> {'View chat history'}
            </HistoryButton>
        );
    }
    return (
        <Header>
            {historyButton}
            {currentTab !== 'new' && (
                <NewChatButton
                    className='new-button'
                    onClick={() => {
                        setCurrentTab('new');
                        selectPost('');
                    }}
                >
                    <i className='icon icon-pencil-outline'/> {'New chat'}
                </NewChatButton>
            )}
        </Header>
    );
};

export default React.memo(RHSHeader);
