import React from 'react';
import styled from 'styled-components';

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

const ExecuteButton = styled.button`
    background: var(--button-bg);
    color: var(--button-color);
    border: none;
    border-radius: 4px;
    padding: 8px 16px;
    cursor: pointer;
    font-weight: 600;

    &:hover {
        background: var(--button-bg-hover);
    }
`;

const ActionContent = styled.pre`
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
    return (
        <ActionBlockContainer>
            <ActionHeader>
                <ActionTitle>Actions</ActionTitle>
                <ExecuteButton onClick={onExecute}>
                    Execute
                </ExecuteButton>
            </ActionHeader>
            <ActionContent>
                {content}
            </ActionContent>
        </ActionBlockContainer>
    );
};

export default ActionBlock;
