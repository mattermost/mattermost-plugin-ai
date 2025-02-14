import React, {useState} from 'react';
import styled from 'styled-components';

import MicroactionDisplay from './microaction_display';
import {client} from '../client/client';

const ActionBlockContainer = styled.div`
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.16);
    border-radius: 4px;
    padding: 12px;
    margin: 8px 0;
    background: rgba(var(--center-channel-color-rgb), 0.04);
`;

const ActionHeader = styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
`;

const ActionTitle = styled.span`
    font-weight: 600;
    color: var(--center-channel-color);
`;

const ExecuteButton = styled.button<{isExecuting?: boolean}>`
    background: var(--button-bg);
    color: var(--button-color);
    border: none;
    border-radius: 4px;
    padding: 8px 16px;
    cursor: ${props => props.isExecuting ? 'wait' : 'pointer'};
    font-weight: 600;
    opacity: ${props => props.isExecuting ? 0.7 : 1};
    position: relative;

    &:hover {
        background: ${props => props.isExecuting ? 'var(--button-bg)' : 'var(--button-bg-hover)'};
    }

    &:disabled {
        cursor: not-allowed;
        opacity: 0.5;
    }
`;

const ErrorMessage = styled.div`
    color: var(--error-text);
    font-size: 12px;
    margin-top: 4px;
`;

const ActionContent = styled.div`
    background: rgba(var(--center-channel-color-rgb), 0.08);
    border-radius: 4px;
    padding: 12px;
    margin: 0;
    overflow-x: auto;
`;

interface Props {
    content: string;
    onExecute: () => void;
}

const ActionBlock: React.FC<Props> = ({content, onExecute}) => {
    const actions = React.useMemo(() => {
        try {
            return JSON.parse(content);
        } catch (e) {
            console.error('Failed to parse actions:', e);
            return [];
        }
    }, [content]);

    if (!Array.isArray(actions)) {
        return (
            <ActionBlockContainer>
                <ActionHeader>
                    <ActionTitle>Invalid Actions Format</ActionTitle>
                </ActionHeader>
                <ActionContent>
                    <PayloadContent>{content}</PayloadContent>
                </ActionContent>
            </ActionBlockContainer>
        );
    }

    return (
        <ActionBlockContainer>
            <ActionHeader>
                <ActionTitle>{`Actions (${actions.length})`}</ActionTitle>
                <div>
                    <ExecuteButton 
                        onClick={handleExecute}
                        disabled={isExecuting}
                        isExecuting={isExecuting}
                    >
                        {isExecuting ? 'Executing...' : 'Execute'}
                    </ExecuteButton>
                    {error && <ErrorMessage>{error}</ErrorMessage>}
                </div>
            </ActionHeader>
            <ActionContent>
                {actions.map((action, index) => (
                    <MicroactionDisplay 
                        key={index}
                        action={action}
                    />
                ))}
            </ActionContent>
        </ActionBlockContainer>
    );
};

export default ActionBlock;
