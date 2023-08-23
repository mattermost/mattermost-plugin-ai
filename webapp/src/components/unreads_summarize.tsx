import React, {useState, useCallback} from 'react';
import styled from 'styled-components';

import {summarizeChannelSince, summarizeThreadSince} from '../client';

import DotMenu from './dot_menu';

import IconAI from './assets/icon_ai';
import LoadingSpinner from './assets/loading_spinner';
import {SubtlePrimaryButton, TertiaryButton} from './assets/buttons';

export const Button = styled(DotMenu)`
    &&&&& {
        background: rgba(255,255,255,0.12);
        margin-left: 10px;
        padding: 2px 10px;
        display: inline-flex;
        align-items: center;
        margin-bottom: 2px;
        width: auto;
        pointer-events: auto;
        color: rgba(var(--new-message-separator-rgb), 1);
        background: rgba(var(--new-message-separator-rgb), 0.08);
        svg {
            margin-right: 4px;
            width: 16px;
            height: 16px;
            fill: rgba(var(--new-message-separator-rgb), 1);
        }
        > span {
            display: flex;
            align-items: center;
        }
    }
`;

export const MenuContent = styled.div`
 && {
     display: flex;
     align-items: center;
     justify-content: center;
     padding: 10px 20px;
     max-width: 500px;
}
`;

export const AIPrimaryButton = styled(SubtlePrimaryButton)`
    height: 24px;
    padding: 0 10px;
    margin-right: 10px;
`;

export const MenuContentButtons = styled.div`
 && {
     display: inline-flex
     align-items: center;
     justify-content: center;
     margin-top: 10px;
}
`;

export const AISecondaryButton = styled(TertiaryButton)`
    height: 24px;
    padding: 0 10px;
    margin-right: 10px;
    background: rgba(var(--center-channel-color-rgb), 0.08);
    color: rgba(var(--center-channel-color-rgb), 0.72);
    fill: rgba(var(--center-channel-color-rgb), 0.72);
    &:hover {
        background: rgba(var(--center-channel-color-rgb), 0.12);
        color: rgba(var(--center-channel-color-rgb), 0.76);
        fill: rgba(var(--center-channel-color-rgb), 0.76);
    }
`;

type SummaryProps = {
    text: string,
    onRegenerate: () => void,
}

const Summary = (props: SummaryProps) => {
    return (
        <div>
            <div>{props.text}</div>
            <MenuContentButtons>
                <AISecondaryButton
                    onClick={(e) => {
                        e.stopPropagation();
                        e.preventDefault();
                        props.onRegenerate();
                    }}
                ><span className='icon'><i className='icon-refresh'/></span>{'Regenerate'}</AISecondaryButton>
            </MenuContentButtons>
        </div>
    );
};

type Props = {
    lastViewedAt: number
    channelId: string
    threadId: string
}

const UnreadsSummarize = ({channelId, threadId, lastViewedAt}: Props) => {
    const [summary, setSummary] = useState<null|string>(null);
    const [generating, setGenerating] = useState(true);
    const [error, setError] = useState('');

    const generateSummary = useCallback(async () => {
        setGenerating(true);
        setSummary('');
        setError('');
        let data;
        try {
            if (channelId) {
                data = await summarizeChannelSince(channelId, lastViewedAt);
            } else if (threadId) {
                data = await summarizeThreadSince(threadId, lastViewedAt);
            }
        } catch (err) {
            setError('Unable to summarize the unread posts');
        }
        setSummary(data.message);
        setGenerating(false);
    }, [channelId, threadId, lastViewedAt]);

    return (
        <Button
            icon={<span><IconAI/>{' Summarize'}</span>}
            title='Summarize'
            onOpenChange={(isOpen) => {
                setSummary('');
                setGenerating(false);
                setError('');
                if (isOpen) {
                    generateSummary();
                }
            }}
        >
            <MenuContent
                onClick={(e) => {
                    if (!(e.target as HTMLElement).classList.contains('ai-error-cancel') && !(e.target as HTMLElement).classList.contains('ai-use-it-button')) {
                        e.stopPropagation();
                        e.preventDefault();
                    }
                }}
            >
                {generating && <LoadingSpinner/>}
                {!generating && error &&
                    <div>
                        <div>{error}</div>
                        <MenuContentButtons>
                            <AIPrimaryButton
                                onClick={generateSummary}
                            >{'Try again'}</AIPrimaryButton>
                            <AISecondaryButton className='ai-error-cancel'>{'Cancel'}</AISecondaryButton>
                        </MenuContentButtons>
                    </div>
                }
                {!error && summary &&
                    <Summary
                        text={summary}
                        onRegenerate={generateSummary}
                    />}
            </MenuContent>
        </Button>
    );
};

export default UnreadsSummarize;
