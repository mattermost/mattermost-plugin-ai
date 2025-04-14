// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import styled from 'styled-components';
import {FormattedMessage} from 'react-intl';
import {ChevronDownIcon, ChevronRightIcon} from '@mattermost/compass-icons/components';

import {ToolCall, ToolCallStatus} from './llmbot_post';

import LoadingSpinner from './assets/loading_spinner';
import IconTool from './assets/icon_tool';
import IconCheckCircle from './assets/icon_check_circle';

// Styled components based on the Figma design
const ToolCallCard = styled.div`
    display: flex;
    flex-direction: column;
    padding: 12px 16px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.08);
    border-radius: 4px;
    background: var(--center-channel-bg);
    box-shadow: 0px 1px 2px 0px rgba(0, 0, 0, 0.08);
    margin-bottom: 12px;
`;

const ToolCallHeader = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
    cursor: pointer;
    user-select: none;
`;

const StyledChevronIcon = styled.div`
    color: rgba(var(--center-channel-color-rgb), 0.56);
    min-width: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
`;

const ToolIcon = styled(IconTool)`
    color: rgba(var(--center-channel-color-rgb), 0.64);
    min-width: 16px;
`;

const ToolName = styled.span`
    font-size: 11px;
    font-weight: 400;
    line-height: 16px;
    letter-spacing: 0.01em;
    color: rgba(var(--center-channel-color-rgb), 0.72);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex-grow: 1;
`;

const ToolCallDescription = styled.div`
    margin: 4px 0;
    font-size: 14px;
    color: rgba(var(--center-channel-color-rgb), 0.76);
`;

const ToolCallArguments = styled.pre`
    margin: 8px 0 12px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
    padding: 12px;
    border-radius: 4px;
    overflow-x: auto;
    font-size: 12px;
    line-height: 1.4;
`;

const StatusContainer = styled.div`
    display: flex;
    align-items: center;
    font-size: 11px;
    line-height: 16px;
    gap: 8px;
    color: rgba(var(--center-channel-color-rgb), 0.75);
    margin-top: 16px;
`;

const ProcessingSpinnerContainer = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
    width: 12px;
    height: 12px;
`;

const ProcessingSpinner = styled(LoadingSpinner)`
    width: 12px;
    height: 12px;
`;

const SuccessIcon = styled(IconCheckCircle)`
    color: var(--online-indicator);
    min-width: 12px;
`;

const ButtonContainer = styled.div`
    display: flex;
    gap: 8px;
    margin-top: 16px;
`;

const ApproveButton = styled.button`
    background: var(--button-bg);
    color: var(--button-color);
    border: none;
    padding: 8px 16px;
    border-radius: 4px;
    font-size: 12px;
    font-weight: 600;
    line-height: 16px;
    cursor: pointer;
    flex: 1;
    
    &:hover {
        background: rgba(var(--button-bg-rgb), 0.88);
    }
    
    &:active {
        background: rgba(var(--button-bg-rgb), 0.92);
    }
`;

const RejectButton = styled.button`
    background: transparent;
    color: var(--error-text);
    border: 1px solid var(--error-text);
    padding: 8px 16px;
    border-radius: 4px;
    font-size: 12px;
    font-weight: 600;
    line-height: 16px;
    cursor: pointer;
    flex: 1;
    
    &:hover {
        background: rgba(var(--error-text-color-rgb), 0.08);
    }
`;

const ResultContainer = styled.pre`
    margin: 8px 0 0;
    padding: 12px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
    border-radius: 4px;
    overflow-x: auto;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.4;
`;

interface ToolCardProps {
    tool: ToolCall;
    isCollapsed: boolean;
    isProcessing: boolean;
    onToggleCollapse: () => void;
    onApprove?: () => void;
    onReject?: () => void;
}

const ToolCard: React.FC<ToolCardProps> = ({
    tool,
    isCollapsed,
    isProcessing,
    onToggleCollapse,
    onApprove,
    onReject,
}) => {
    const isPending = tool.status === ToolCallStatus.Pending;
    const isAccepted = tool.status === ToolCallStatus.Accepted;
    const isSuccess = tool.status === ToolCallStatus.Success;
    const isError = tool.status === ToolCallStatus.Error;
    const isRejected = tool.status === ToolCallStatus.Rejected;

    return (
        <ToolCallCard>
            <ToolCallHeader onClick={onToggleCollapse}>
                <StyledChevronIcon>
                    {isCollapsed ? <ChevronRightIcon size={16}/> : <ChevronDownIcon size={16}/>}
                </StyledChevronIcon>
                <ToolIcon/>
                <ToolName>{tool.name}</ToolName>
            </ToolCallHeader>

            {!isCollapsed && (
                <>
                    <ToolCallDescription>{tool.description}</ToolCallDescription>
                    <ToolCallArguments>{JSON.stringify(tool.arguments, null, 2)}</ToolCallArguments>

                    {isPending && (
                        isProcessing ? (
                            <StatusContainer>
                                <ProcessingSpinnerContainer>
                                    <ProcessingSpinner/>
                                </ProcessingSpinnerContainer>
                                <FormattedMessage
                                    id='ai.tool_call.processing'
                                    defaultMessage='Processing...'
                                />
                            </StatusContainer>
                        ) : (
                            <ButtonContainer>
                                <ApproveButton onClick={onApprove}>
                                    <FormattedMessage
                                        id='ai.tool_call.approve'
                                        defaultMessage='Approve'
                                    />
                                </ApproveButton>
                                <RejectButton onClick={onReject}>
                                    <FormattedMessage
                                        id='ai.tool_call.reject'
                                        defaultMessage='Reject'
                                    />
                                </RejectButton>
                            </ButtonContainer>
                        )
                    )}

                    {isAccepted && (
                        <StatusContainer>
                            <ProcessingSpinnerContainer>
                                <ProcessingSpinner/>
                            </ProcessingSpinnerContainer>
                            <FormattedMessage
                                id='ai.tool_call.status.processing'
                                defaultMessage='Processing...'
                            />
                        </StatusContainer>
                    )}

                    {isSuccess && (
                        <>
                            <StatusContainer>
                                <SuccessIcon/>
                                <FormattedMessage
                                    id='ai.tool_call.status.complete'
                                    defaultMessage='Complete'
                                />
                            </StatusContainer>
                            {tool.result && <ResultContainer>{tool.result}</ResultContainer>}
                        </>
                    )}

                    {isError && (
                        <>
                            <StatusContainer>
                                <span style={{color: 'var(--error-text)'}}>{'⚠️'}</span>
                                <FormattedMessage
                                    id='ai.tool_call.status.error'
                                    defaultMessage='Error'
                                />
                            </StatusContainer>
                            {tool.result && <ResultContainer>{tool.result}</ResultContainer>}
                        </>
                    )}

                    {isRejected && (
                        <StatusContainer>
                            <span>{'🚫'}</span>
                            <FormattedMessage
                                id='ai.tool_call.status.rejected'
                                defaultMessage='Rejected'
                            />
                        </StatusContainer>
                    )}
                </>
            )}
        </ToolCallCard>
    );
};

export default ToolCard;
