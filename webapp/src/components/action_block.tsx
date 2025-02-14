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

const ActionContent = styled.div`
    background: rgba(var(--center-channel-color-rgb), 0.08);
    border-radius: 4px;
    padding: 12px;
    margin: 0;
    overflow-x: auto;
`;

const ActionItem = styled.div`
    margin-bottom: 12px;
    padding: 8px;
    background: var(--center-channel-bg);
    border-radius: 4px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.08);

    &:last-child {
        margin-bottom: 0;
    }
`;

const ActionName = styled.div`
    font-weight: 600;
    color: var(--center-channel-color);
    margin-bottom: 8px;
`;

const PayloadContent = styled.pre`
    margin: 0;
    padding: 8px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
    border-radius: 4px;
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
                <ExecuteButton onClick={onExecute}>
                    Execute
                </ExecuteButton>
            </ActionHeader>
            <ActionContent>
                {actions.map((action, index) => (
                    <ActionItem key={index}>
                        <ActionName>{action.action}</ActionName>
                        <PayloadContent>
                            {JSON.stringify(action.payload, null, 2)}
                        </PayloadContent>
                    </ActionItem>
                ))}
            </ActionContent>
        </ActionBlockContainer>
    );
};

export default ActionBlock;
