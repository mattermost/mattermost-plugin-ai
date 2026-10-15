// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage} from 'react-intl';
import {CheckIcon, CloseCircleOutlineIcon, OpenInNewIcon, PowerPlugOutlineIcon} from '@mattermost/compass-icons/components';

import {Button} from '@mattermost/compass-ui/components/button';
import {Icon} from '@mattermost/compass-ui/components/icon';
import {Spinner} from '@mattermost/compass-ui/components/spinner';

import {getUserMCPTools, type UserMCPServerInfo} from '@/client';
import {useMCPConnectionEvents} from '@/hooks/use_mcp_connection_events';

import {ToolCall, ToolCallStatus} from './tool_types';

// Parsed shape of the RequestMCPConnection tool arguments. Mirrors
// mcp.RequestConnectionArgs on the server.
export interface ConnectionRequestArgs {
    server: string;
    reason: string;
}

// parseConnectionRequestArgs returns null when the arguments are missing or
// malformed (e.g. redacted for non-requesters) so the caller can fall back to
// the generic tool card.
export function parseConnectionRequestArgs(args: ToolCall['arguments']): ConnectionRequestArgs | null {
    if (args == null || typeof args !== 'object' || Array.isArray(args)) {
        return null;
    }
    const obj = args as {[key: string]: unknown};
    if (typeof obj.server !== 'string' || obj.server.trim() === '') {
        return null;
    }
    return {
        server: obj.server.trim(),
        reason: typeof obj.reason === 'string' ? obj.reason.trim() : '',
    };
}

// connectedFromResult reads the connected flag from the tool result
// (mcp.RequestConnectionResult on the server).
function connectedFromResult(result?: string): boolean {
    if (!result) {
        return false;
    }
    try {
        return JSON.parse(result)?.connected === true;
    } catch {
        return false;
    }
}

type Lookup =
    | {state: 'loading'}
    | {state: 'failed'}
    | {state: 'loaded'; server?: UserMCPServerInfo};

interface ConnectionRequestCardProps {
    tool: ToolCall;
    request: ConnectionRequestArgs;
    isProcessing: boolean;
    localDecision?: boolean;
    canAnswer: boolean;
    onContinue?: () => void;
    onSkip?: () => void;
}

const ConnectionRequestCard: React.FC<ConnectionRequestCardProps> = ({
    tool,
    request,
    isProcessing,
    localDecision,
    canAnswer,
    onContinue,
    onSkip,
}) => {
    const isPending = tool.status === ToolCallStatus.Pending || tool.status === ToolCallStatus.Accepted;
    const isAnswered = tool.status === ToolCallStatus.Success;
    const isSkipped = tool.status === ToolCallStatus.Rejected;
    const hasLocalDecision = localDecision != null;
    const interactive = isPending && canAnswer && !isProcessing && !hasLocalDecision && Boolean(onContinue && onSkip);

    const [lookup, setLookup] = useState<Lookup>({state: 'loading'});
    const [connectOpened, setConnectOpened] = useState(false);
    const latestFetchRef = useRef(0);

    const fetchServer = useCallback(async () => {
        const fetchID = ++latestFetchRef.current;
        try {
            const response = await getUserMCPTools();
            if (fetchID === latestFetchRef.current) {
                setLookup({state: 'loaded', server: response.servers.find((s) => s.name === request.server)});
            }
        } catch {
            if (fetchID === latestFetchRef.current) {
                setLookup({state: 'failed'});
            }
        }
    }, [request.server]);

    useEffect(() => {
        if (interactive) {
            fetchServer();
        }
    }, [interactive, fetchServer]);

    useMCPConnectionEvents(useCallback((event) => {
        if (interactive && (!event.serverName || event.serverName === request.server)) {
            fetchServer();
        }
    }, [interactive, request.server, fetchServer]));

    // The websocket event can be missed (e.g. during a reconnect), so also
    // re-check when the user comes back from the sign-in window.
    useEffect(() => {
        const refreshOnReturn = () => {
            if (interactive && connectOpened && document.visibilityState === 'visible') {
                fetchServer();
            }
        };
        window.addEventListener('focus', refreshOnReturn);
        document.addEventListener('visibilitychange', refreshOnReturn);
        return () => {
            window.removeEventListener('focus', refreshOnReturn);
            document.removeEventListener('visibilitychange', refreshOnReturn);
        };
    }, [interactive, connectOpened, fetchServer]);

    const answeredConnected = useMemo(() => connectedFromResult(tool.result), [tool.result]);

    const handleConnect = (authURL: string) => {
        window.open(authURL, '_blank', 'noopener,noreferrer');
        setConnectOpened(true);
    };

    const renderInteractiveFooter = () => {
        if (lookup.state === 'loading') {
            return (
                <StatusLine>
                    <Spinner
                        size='12'
                        aria-hidden={true}
                    />
                    <FormattedMessage
                        id='ai.connection_request.checking'
                        defaultMessage='Checking your connection…'
                    />
                </StatusLine>
            );
        }

        const server = lookup.state === 'loaded' ? lookup.server : undefined; // eslint-disable-line no-undefined
        const connected = Boolean(server?.authenticated);
        const authURL = server && !connected ? server.authURL : undefined; // eslint-disable-line no-undefined
        const unavailable = lookup.state === 'loaded' && !connected && !authURL;

        let status: React.ReactNode = null;
        if (connected) {
            status = (
                <StatusLine>
                    <ConnectedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connection_request.connected_continue'
                        defaultMessage='Connected. Continue to let the agent use it.'
                    />
                </StatusLine>
            );
        } else if (unavailable) {
            status = (
                <StatusLine>
                    <FormattedMessage
                        id='ai.connection_request.unavailable'
                        defaultMessage="This integration can't be connected from here. Ask your system admin for access."
                    />
                </StatusLine>
            );
        } else if (connectOpened) {
            status = (
                <StatusLine>
                    <FormattedMessage
                        id='ai.connection_request.finish_in_window'
                        defaultMessage='Finish connecting in the new window, then continue.'
                    />
                </StatusLine>
            );
        }

        // Continue is offered whenever connecting may have happened, but only
        // emphasized once the connection is seen: continuing early resolves the
        // request as not connected. The server re-checks either way.
        const showContinue = connected || connectOpened || lookup.state === 'failed';

        return (
            <>
                {status}
                <Footer>
                    <Button
                        emphasis='tertiary'
                        size='small'
                        onClick={onSkip}
                    >
                        <FormattedMessage
                            id='ai.connection_request.skip'
                            defaultMessage='Skip'
                        />
                    </Button>
                    {authURL && (
                        <Button
                            emphasis='primary'
                            size='small'
                            trailingIcon={<Icon glyph={<OpenInNewIcon/>}/>}
                            onClick={() => handleConnect(authURL)}
                        >
                            <FormattedMessage
                                id='ai.connection_request.connect'
                                defaultMessage='Connect'
                            />
                        </Button>
                    )}
                    {showContinue && (
                        <Button
                            emphasis={connected ? 'primary' : 'secondary'}
                            size='small'
                            onClick={onContinue}
                        >
                            <FormattedMessage
                                id='ai.connection_request.continue'
                                defaultMessage='Continue'
                            />
                        </Button>
                    )}
                </Footer>
            </>
        );
    };

    const renderStatus = () => {
        if (isProcessing || (hasLocalDecision && isPending)) {
            return (
                <StatusLine>
                    <Spinner
                        size='12'
                        aria-hidden={true}
                    />
                    <FormattedMessage
                        id='ai.connection_request.submitting'
                        defaultMessage='Submitting…'
                    />
                </StatusLine>
            );
        }
        if (isAnswered) {
            return answeredConnected ? (
                <StatusLine>
                    <ConnectedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connection_request.connected'
                        defaultMessage='Connected'
                    />
                </StatusLine>
            ) : (
                <StatusLine>
                    <SkippedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connection_request.not_connected'
                        defaultMessage='Not connected'
                    />
                </StatusLine>
            );
        }
        if (isSkipped) {
            return (
                <StatusLine>
                    <SkippedIcon size={16}/>
                    <FormattedMessage
                        id='ai.connection_request.skipped'
                        defaultMessage='Skipped'
                    />
                </StatusLine>
            );
        }
        if (isPending && !canAnswer) {
            return (
                <StatusLine>
                    <FormattedMessage
                        id='ai.connection_request.waiting_for_requester'
                        defaultMessage='Waiting for the requester to connect'
                    />
                </StatusLine>
            );
        }
        return null;
    };

    return (
        <Card data-testid='connection-request-card'>
            <Header>
                <HeaderIcon>
                    <PowerPlugOutlineIcon size={18}/>
                </HeaderIcon>
                <Title>
                    <FormattedMessage
                        id='ai.connection_request.title'
                        defaultMessage='Connect {server}'
                        values={{server: request.server}}
                    />
                </Title>
            </Header>
            {request.reason && <Reason>{request.reason}</Reason>}
            {isPending && (
                <Hint>
                    <FormattedMessage
                        id='ai.connection_request.hint'
                        defaultMessage='You sign in to {server} with your own account. The agent can then use it on your behalf in this conversation.'
                        values={{server: request.server}}
                    />
                </Hint>
            )}
            {interactive ? renderInteractiveFooter() : renderStatus()}
        </Card>
    );
};

const Card = styled.div`
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 8px;
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

const Header = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
    padding-left: 12px;
`;

const HeaderIcon = styled.span`
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 4px;
    background: rgba(var(--button-bg-rgb), 0.08);
    color: var(--button-bg);
    flex-shrink: 0;
`;

const Title = styled.div`
    font-size: 14px;
    font-weight: 600;
    line-height: 20px;
    color: var(--center-channel-color);
    word-break: break-word;
`;

const Reason = styled.div`
    padding-left: 12px;
    font-size: 14px;
    line-height: 20px;
    color: var(--center-channel-color);
    word-break: break-word;
`;

const Hint = styled.div`
    padding-left: 12px;
    font-size: 12px;
    line-height: 16px;
    color: rgba(var(--center-channel-color-rgb), 0.64);
`;

const Footer = styled.div`
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 4px;
`;

const StatusLine = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
    padding-left: 12px;
    font-size: 12px;
    line-height: 16px;
    color: rgba(var(--center-channel-color-rgb), 0.75);
`;

const ConnectedIcon = styled(CheckIcon)`
    color: var(--online-indicator);
`;

const SkippedIcon = styled(CloseCircleOutlineIcon)`
    color: var(--dnd-indicator);
`;

export default ConnectionRequestCard;
