// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import styled from 'styled-components';

import {Timestamp} from '@/mm_webapp';

import {GrayPill} from '../pill';

const ThreadItemContainer = styled.div`
    padding: 16px;
    cursor: pointer;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12)
`;

const Title = styled.div`
    color: var(--center-channel-color);
    display: flex;
    align-items: center;
    margin-bottom: 4px;
    justify-content: space-between;
`;

const TitleText = styled.div`
    font-size: 14px;
    font-weight: 600;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
`;

const Preview = styled.div`
    overflow: hidden;
    color: var(--center-channel-color);
    text-overflow: ellipsis;
    whitespace: nowrap;
    margin-bottom: 12px;
	height: 40px;
	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
`;

const RepliesCount = styled.div`
    color: rgba(var(--center-channel-color-rgb), 0.64);
    font-weight: 600;
`;

const LastActivityDate = styled.div`
    color: rgba(var(--center-channel-color-rgb), 0.64);
    font-size: 12px;
    font-weight: 400;
    white-space: nowrap;
    margin-left: 13px;
`;

const Label = styled(GrayPill)`
	padding: 0 4px;
	font-size: 10px;
	font-weight: 600;
	line-height: 16px;
`;

const Footer = styled.div`
	display: flex;
	flex-direction: row;
	gap: 10px;
`;

type Props = {
    postTitle: string;
    postMessage: string;
    repliesCount: number;
    lastActivityDate: number;
    label: string;
    onClick: () => void;
}

const DefaultTitle = 'Conversation with Copilot';

export default function ThreadItem(props: Props) {
    const repliesText = props.repliesCount === 1 ? '1 reply' : `${props.repliesCount} replies`;
    return (
        <ThreadItemContainer onClick={props.onClick}>
            <Title>
                <TitleText>{props.postTitle || DefaultTitle}</TitleText>
                <LastActivityDate>
                    <Timestamp // Matches the timestap format in the threads view
                        value={props.lastActivityDate}
                        units={['now', 'minute', 'hour', 'day', 'week']}
                        useTime={false}
                        day={'numeric'}
                    />
                </LastActivityDate>
            </Title>
            <Preview>{props.postMessage}</Preview>
            <Footer>
                <Label>{props.label}</Label>
                <RepliesCount>{repliesText}</RepliesCount>
            </Footer>
        </ThreadItemContainer>
    );
}
