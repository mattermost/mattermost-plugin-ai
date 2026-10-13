// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage} from 'react-intl';
import {CheckIcon, CloseCircleOutlineIcon, AlertCircleOutlineIcon} from '@mattermost/compass-icons/components';

import {Button} from '@mattermost/compass-ui/components/button';
import {Spinner} from '@mattermost/compass-ui/components/spinner';

import {getUserMCPTools} from '@/client';
import {useMCPConnectionEvents} from '@/hooks/use_mcp_connection_events';

import {ToolCall, ToolCallStatus} from './tool_types';

// Parsed shape of the ConnectMCPServer tool arguments. Mirrors the server-side
// ConnectMCPServer args ({"server": "<configured server name>"}).
export interface ConnectArgs {
    server: string;
}

// parseConnectArgs extracts the server name from tool call arguments. Returns
// null when the arguments are missing or malformed (e.g. redacted for
// non-requesters).
export function parseConnectArgs(args: ToolCall['arguments']): ConnectArgs | null {
    if (args == null || typeof args !== 'object' || Array.isArray(args)) {
        return null;
    }
    const server = (args as {[key: string]: unknown}).server;
    if (typeof server !== 'string' || server.trim() === '') {
        return null;
    }
    return {server};
}

type ConnectCardState = 'interactive' | 'submitting' | 'waiting' | 'connected' | 'skipped' | 'failed';

function computeState(status: ToolCallStatus, canAnswer: boolean, busy: boolean): ConnectCardState {
    switch (status) {
    case ToolCallStatus.Pending:
        if (busy) {
            return 'submitting';
        }
        return canAnswer ? 'interactive' : 'waiting';
    case ToolCallStatus.Accepted:
    case ToolCallStatus.Success:
    case ToolCallStatus.AutoApproved:
        return 'connected';
    case ToolCallStatus.Rejected:
        return 'skipped';
    case ToolCallStatus.Error:
        return 'failed';
    default: {
        const unreachable: never = status;
        return unreachable;
    }
    }
}

type ServerStatus = {
    authenticated: boolean;
    authURL?: string;
};

async function fetchServerStatus(server: string): Promise<ServerStatus | null> {
    const response = await getUserMCPTools();
    const match = response.servers.find((s) => s.name === server);
    if (!match) {
        return null;
    }
    return {authenticated: match.authenticated, authURL: match.authURL};
}

const Card = styled.div`
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 16px 16px 12px;
    margin: 8px 0 12px;
    overflow: hidden;
    background: var(--center-channel-bg);
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    border-radius: 4px;
    box-shadow: 0 2px 3px rgba(0, 0, 0, 0.08);

    &::before {
        content: '';
        position: absolute;
        top: 0;
        left: 0;
        bottom: 0;
        width: 3px;
        background: var(--button-bg);
    }
`;

const Title = styled.div`
    padding-left: 12px;
    font-size: 14px;
    font-weight: 600;
    line-height: 20px;
    color: var(--center-channel-color);
    word-break: break-word;
`;

const ServerName = styled.span`
    font-weight: 600;
`;

const Explanation = styled.div`
    padding-left: 12px;
    font-size: 14px;
    font-weight: 400;
    line-height: 20px;
    color: rgba(var(--center-channel-color-rgb), 0.75);
    word-break: break-word;
`;

const Footer = styled.div`
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding-left: 12px;
    padding-top: 4px;
`;

const StatusLine = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
    padding-left: 12px;
    font-size: 12px;
    font-weight: 400;
    line-height: 16px;
    color: rgba(var(--center-channel-color-rgb), 0.75);
`;

const ConnectedIcon = styled(CheckIcon)`
    color: var(--online-indicator);
`;

const SkippedIcon = styled(CloseCircleOutlineIcon)`
    color: var(--dnd-indicator);
`;

const FailedIcon = styled(AlertCircleOutlineIcon)`
    color: var(--error-text);
`;

interface ConnectCardProps {
    tool: ToolCall;

    // Null when the arguments are redacted (non-requesters).
    server: string | null;
    isProcessing: boolean;
    localDecision?: boolean;
    canAnswer: boolean;
    onAccept?: () => void;
    onSkip?: () => void;
}

const ConnectCard: React.FC<ConnectCardProps> = ({
    tool,
    server,
    isProcessing,
    localDecision,
    canAnswer,
    onAccept,
    onSkip,
}) => {
    const busy = isProcessing || localDecision != null;
    const state = computeState(tool.status, canAnswer && Boolean(server && onAccept && onSkip), busy);
    const interactive = state === 'interactive';

    const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null);
    const [statusLoaded, setStatusLoaded] = useState(false);
    const [connectStarted, setConnectStarted] = useState(false);

    // Refs so the websocket listener stays stable while reading fresh values.
    const connectStartedRef = useRef(false);
    const autoSubmittedRef = useRef(false);
    const interactiveRef = useRef(interactive);
    const onAcceptRef = useRef(onAccept);
    interactiveRef.current = interactive;
    onAcceptRef.current = onAccept;

    const refreshStatus = useCallback(async (): Promise<ServerStatus | null> => {
        if (!server) {
            return null;
        }
        try {
            const next = await fetchServerStatus(server);
            setServerStatus(next);
            return next;
        } catch {
            return null;
        } finally {
            setStatusLoaded(true);
        }
    }, [server]);

    useEffect(() => {
        if (interactive) {
            refreshStatus();
        }
    }, [interactive, refreshStatus]);

    useMCPConnectionEvents(useCallback(() => {
        if (!interactiveRef.current) {
            return;
        }
        refreshStatus().then((next) => {
            if (!next?.authenticated || !connectStartedRef.current || autoSubmittedRef.current || !interactiveRef.current) {
                return;
            }
            autoSubmittedRef.current = true;
            onAcceptRef.current?.();
        });
    }, [refreshStatus]));

    const handleConnect = () => {
        if (!serverStatus?.authURL) {
            return;
        }
        connectStartedRef.current = true;
        setConnectStarted(true);
        window.open(serverStatus.authURL, '_blank', 'noopener,noreferrer');
    };

    const handleContinue = () => {
        autoSubmittedRef.current = true;
        onAccept?.();
    };

    const authenticated = serverStatus?.authenticated === true;
    const authURL = serverStatus?.authURL;
    const showConnect = !authenticated;
    const showContinue = authenticated || connectStarted || (statusLoaded && !authURL);

    const renderTitle = () => {
        if (!server) {
            return (
                <FormattedMessage
                    id='ai.connect_mcp.title_unknown'
                    defaultMessage='Connect a tool provider'
                />
            );
        }
        return (
            <FormattedMessage
                id='ai.connect_mcp.title'
                defaultMessage='Connect <name>{server}</name>'
                values={{
                    server,
                    name: (chunks: React.ReactNode) => (
                        <ServerName
                            key='server'
                            data-testid='connect-mcp-server-name'
                        >
                            {chunks}
                        </ServerName>
                    ),
                }}
            />
        );
    };

    const renderExplanation = () => {
        if (!interactive) {
            return null;
        }
        if (authenticated) {
            return (
                <FormattedMessage
                    id='ai.connect_mcp.already_connected'
                    defaultMessage='{server} is connected. Continue so the agent can use it for your request.'
                    values={{server}}
                />
            );
        }
        if (statusLoaded && !authURL) {
            return (
                <FormattedMessage
                    id='ai.connect_mcp.no_auth_url'
                    defaultMessage='A connection link for {server} is not available. Connect it from the tools menu, then continue.'
                    values={{server}}
                />
            );
        }
        if (connectStarted) {
            return (
                <FormattedMessage
                    id='ai.connect_mcp.in_progress'
                    defaultMessage='Finish connecting {server} in the new window. The agent continues automatically once you are connected.'
                    values={{server}}
                />
            );
        }
        return (
            <FormattedMessage
                id='ai.connect_mcp.explanation'
                defaultMessage='The agent needs {server} to continue with your request, but your account is not connected to it yet.'
                values={{server}}
            />
        );
    };

    const renderStatus = () => {
        switch (state) {
        case 'interactive':
            return null;
        case 'submitting':
            return (
                <StatusLine>
                    <Spinner
                        size='12'
                        aria-hidden={true}
                    />
                    <FormattedMessage
                        id='ai.connect_mcp.submitting'
                        defaultMessage='Submitting…'
                    />
                </StatusLine>
            );
        case 'waiting':
            return (
                <StatusLine data-testid='connect-mcp-waiting'>
                    {server ? (
                        <FormattedMessage
                            id='ai.connect_mcp.waiting_for_requester'
                            defaultMessage='Waiting for the requester to connect {server}'
                            values={{server}}
                        />
                    ) : (
                        <FormattedMessage
                            id='ai.connect_mcp.waiting_for_requester_unknown'
                            defaultMessage='Waiting for the requester to connect a tool provider'
                        />
                    )}
                </StatusLine>
            );
        case 'connected':
            return (
                <StatusLine data-testid='connect-mcp-connected'>
                    <ConnectedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connect_mcp.connected'
                        defaultMessage='Connected'
                    />
                </StatusLine>
            );
        case 'skipped':
            return (
                <StatusLine data-testid='connect-mcp-skipped'>
                    <SkippedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connect_mcp.skipped'
                        defaultMessage='Skipped'
                    />
                </StatusLine>
            );
        case 'failed':
            return (
                <StatusLine data-testid='connect-mcp-failed'>
                    <FailedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connect_mcp.failed'
                        defaultMessage='Could not connect'
                    />
                </StatusLine>
            );
        default: {
            const unreachable: never = state;
            return unreachable;
        }
        }
    };

    const explanation = renderExplanation();

    return (
        <Card data-testid='connect-mcp-card'>
            <Title>{renderTitle()}</Title>
            {explanation && <Explanation>{explanation}</Explanation>}
            {interactive && (
                <Footer>
                    <Button
                        emphasis='tertiary'
                        size='small'
                        onClick={onSkip}
                        data-testid='connect-mcp-skip-button'
                    >
                        <FormattedMessage
                            id='ai.connect_mcp.skip'
                            defaultMessage='Skip'
                        />
                    </Button>
                    {showConnect && (
                        <Button
                            emphasis={showContinue ? 'secondary' : 'primary'}
                            size='small'
                            disabled={!authURL}
                            onClick={handleConnect}
                            data-testid='connect-mcp-connect-button'
                        >
                            <FormattedMessage
                                id='ai.connect_mcp.connect'
                                defaultMessage='Connect'
                            />
                        </Button>
                    )}
                    {showContinue && (
                        <Button
                            emphasis='primary'
                            size='small'
                            onClick={handleContinue}
                            data-testid='connect-mcp-continue-button'
                        >
                            <FormattedMessage
                                id='ai.connect_mcp.continue'
                                defaultMessage='Continue'
                            />
                        </Button>
                    )}
                </Footer>
            )}
            {renderStatus()}
        </Card>
    );
};

export default ConnectCard;
