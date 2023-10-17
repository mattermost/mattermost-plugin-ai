import React from 'react';
import styled from 'styled-components';

const ThreadItemContainer = styled.div`
    padding: 16px;
    cursor: pointer;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12)
`;

const Timestamp = styled((window as any).Components.Timestamp)`
`;

const Title = styled.div`
    color: var(--center-channel-color);
    display: flex;
    align-items: center;
    margin-bottom: 4px;
    justify-content: space-between;
    .title-text {
        font-size: 14px;
        font-weight: 600;
        text-overflow: ellipsis;
        overflow: hidden;
        white-space: nowrap;
    }
`;

const FirstReply = styled.div`
    overflow: hidden;
    color: var(--denim-center-channel-text, #3F4350);
    text-overflow: ellipsis;
    whitespace: nowrap;
    margin-bottom: 12px;
`;

const RepliesCount = styled.div`
    color: rgba(var(--center-channel-color-rgb, 0.64));
`;

const LastActivityDate = styled.div`
    color: rgba(var(--center-channel-color-rgb, 0.64));
    font-size: 12px;
    font-weight: 400;
    white-space: nowrap;
    margin-left: 13px;
`;

type Props = {
    postMessage: string;
    postFirstReply: string;
    repliesCount: number;
    lastActivityDate: number;
    onClick: () => void;
}

export default function ThreadItem(props: Props) {
    return (
        <ThreadItemContainer onClick={props.onClick}>
            <Title>
                <div className='title-text'>{props.postMessage}</div>
                <LastActivityDate><Timestamp value={props.lastActivityDate}/></LastActivityDate>
            </Title>
            <FirstReply>{props.postFirstReply}</FirstReply>
            <RepliesCount>{props.repliesCount}{' replies'}</RepliesCount>
        </ThreadItemContainer>
    );
}
