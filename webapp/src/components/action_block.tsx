import React, {useState} from 'react';
import {useSelector} from 'react-redux';
import styled from 'styled-components';

import {GlobalState} from '@mattermost/types/store';

import MicroactionDisplay from './microaction_display';

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
    onExecute: (actions: any) => void;
    isExecuting: boolean;
    channelID: string;
    executionError: string;
}

const ActionBlock: React.FC<Props> = ({content, onExecute, isExecuting, executionError, channelID}) => {
    const teamID = useSelector<GlobalState, string>((state) => state.entities.teams.currentTeamId);

    const actions = React.useMemo(() => {
        console.log(content)
        let newContent = content.replace(/"?\{\{current_team_id\}\}"?/g, `"${teamID || ''}"`)
        newContent = newContent.replace(/"?\{\{current_channel_id\}\}"?/g, `"${channelID || ''}"`)
        return JSON.parse(newContent);
    }, [content, teamID, channelID]);

    if (!Array.isArray(actions)) {
        return (
            <ActionBlockContainer>
                <ActionHeader>
                    <ActionTitle>Invalid Actions Format</ActionTitle>
                </ActionHeader>
                <ActionContent>{content}</ActionContent>
            </ActionBlockContainer>
        );
    }

    return (
        <ActionBlockContainer>
            <ActionHeader>
                <ActionTitle>{`Actions (${actions.length})`}</ActionTitle>
                <div>
                    <ExecuteButton
                        onClick={() => onExecute(actions)}
                        disabled={isExecuting}
                        isExecuting={isExecuting}
                    >
                        {isExecuting ? 'Executing...' : 'Execute'}
                    </ExecuteButton>
                    {executionError && <ErrorMessage>{executionError}</ErrorMessage>}
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
