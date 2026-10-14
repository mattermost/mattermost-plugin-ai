// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {IntlProvider} from 'react-intl';

import {getUserMCPTools, type UserMCPServerInfo} from '@/client';
import {notifyMCPConnectionUpdated} from '@/hooks/use_mcp_connection_events';

import ConnectionRequestCard, {parseConnectionRequestArgs} from './connection_request_card';
import {ToolCall, ToolCallStatus} from './tool_types';

jest.mock('@/client', () => ({
    getUserMCPTools: jest.fn(),
}));

const mockGetUserMCPTools = getUserMCPTools as jest.MockedFunction<typeof getUserMCPTools>;

const authURL = 'http://localhost:8065/plugins/mattermost-ai/mcp/oauth/Jira/start';

function jiraServer(overrides: Partial<UserMCPServerInfo> = {}): UserMCPServerInfo {
    return {
        name: 'Jira',
        serverOrigin: 'https://jira.example.com/mcp',
        kind: 'remote',
        authenticated: false,
        needsOAuth: true,
        authURL,
        serviceAccountConfigured: false,
        tools: [],
        ...overrides,
    };
}

function makeTool(overrides: Partial<ToolCall> = {}): ToolCall {
    return {
        id: 'connect_1',
        name: 'RequestMCPConnection',
        description: '',
        status: ToolCallStatus.Pending,
        user_interaction: 'connect',
        ...overrides,
    };
}

function renderCard(props: Partial<React.ComponentProps<typeof ConnectionRequestCard>> = {}) {
    const onContinue = props.onContinue ?? jest.fn();
    const onSkip = props.onSkip ?? jest.fn();
    render(
        <IntlProvider locale='en'>
            <ConnectionRequestCard
                tool={props.tool ?? makeTool()}
                request={props.request ?? {server: 'Jira', reason: 'To look up your open issues.'}}
                isProcessing={props.isProcessing ?? false}
                localDecision={props.localDecision}
                canAnswer={props.canAnswer ?? true}
                onContinue={onContinue}
                onSkip={onSkip}
            />
        </IntlProvider>,
    );
    return {onContinue, onSkip};
}

beforeEach(() => {
    mockGetUserMCPTools.mockReset();
});

describe('parseConnectionRequestArgs', () => {
    test.each([
        ['well-formed request', {server: ' Jira ', reason: ' Find issues '}, {server: 'Jira', reason: 'Find issues'}],
        ['missing reason', {server: 'Jira'}, {server: 'Jira', reason: ''}],
        ['missing server', {reason: 'x'}, null],
        ['blank server', {server: '  '}, null],
        ['redacted arguments', undefined, null], // eslint-disable-line no-undefined
        ['array arguments', ['Jira'], null],
    ])('%s', (_name, args, expected) => {
        expect(parseConnectionRequestArgs(args as ToolCall['arguments'])).toEqual(expected);
    });
});

describe('ConnectionRequestCard', () => {
    test('offers Connect for an unconnected server and Continue once the window is opened', async () => {
        mockGetUserMCPTools.mockResolvedValue({servers: [jiraServer()]});
        const openSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
        const {onContinue} = renderCard();

        expect(screen.getByText('Connect Jira')).toBeTruthy();
        expect(screen.getByText('To look up your open issues.')).toBeTruthy();

        const connect = await screen.findByRole('button', {name: /^connect$/i});
        expect(screen.queryByRole('button', {name: /^continue$/i})).toBeNull();

        fireEvent.click(connect);
        expect(openSpy).toHaveBeenCalledWith(authURL, '_blank', 'noopener,noreferrer');

        fireEvent.click(await screen.findByRole('button', {name: /^continue$/i}));
        expect(onContinue).toHaveBeenCalledTimes(1);
        openSpy.mockRestore();
    });

    test('offers Continue without Connect when the server is already connected', async () => {
        mockGetUserMCPTools.mockResolvedValue({servers: [jiraServer({authenticated: true, authURL: undefined})]}); // eslint-disable-line no-undefined
        renderCard();

        expect(await screen.findByText('Connected. Continue to let the agent use it.')).toBeTruthy();
        expect(screen.getByRole('button', {name: /^continue$/i})).toBeTruthy();
        expect(screen.queryByRole('button', {name: /^connect$/i})).toBeNull();
    });

    test('refreshes when the server reports a new connection', async () => {
        mockGetUserMCPTools.mockResolvedValueOnce({servers: [jiraServer()]});
        renderCard();
        await screen.findByRole('button', {name: /^connect$/i});

        mockGetUserMCPTools.mockResolvedValueOnce({servers: [jiraServer({authenticated: true, authURL: undefined})]}); // eslint-disable-line no-undefined
        act(() => {
            notifyMCPConnectionUpdated({status: 'connected', serverName: 'Jira'});
        });

        expect(await screen.findByRole('button', {name: /^continue$/i})).toBeTruthy();
        expect(mockGetUserMCPTools).toHaveBeenCalledTimes(2);
    });

    test('ignores connection events for other servers', async () => {
        mockGetUserMCPTools.mockResolvedValue({servers: [jiraServer()]});
        renderCard();
        await screen.findByRole('button', {name: /^connect$/i});

        act(() => {
            notifyMCPConnectionUpdated({status: 'connected', serverName: 'GitHub'});
        });
        expect(mockGetUserMCPTools).toHaveBeenCalledTimes(1);
    });

    test('explains when the server cannot be connected from here and only allows skipping', async () => {
        mockGetUserMCPTools.mockResolvedValue({servers: []});
        const {onSkip} = renderCard();

        expect(await screen.findByText(/can't be connected from here/)).toBeTruthy();
        expect(screen.queryByRole('button', {name: /^connect$/i})).toBeNull();
        expect(screen.queryByRole('button', {name: /^continue$/i})).toBeNull();

        fireEvent.click(screen.getByRole('button', {name: /^skip$/i}));
        expect(onSkip).toHaveBeenCalledTimes(1);
    });

    test('still lets the user continue when the connection lookup fails', async () => {
        mockGetUserMCPTools.mockRejectedValue(new Error('network'));
        renderCard();

        expect(await screen.findByRole('button', {name: /^continue$/i})).toBeTruthy();
        expect(screen.getByRole('button', {name: /^skip$/i})).toBeTruthy();
    });

    test.each([
        ['connected result', makeTool({status: ToolCallStatus.Success, result: '{"server":"Jira","connected":true}'}), true, 'Connected'],
        ['not connected result', makeTool({status: ToolCallStatus.Success, result: '{"server":"Jira","connected":false}'}), true, 'Not connected'],
        ['skipped request', makeTool({status: ToolCallStatus.Rejected}), true, 'Skipped'],
        ['pending request seen by another user', makeTool(), false, 'Waiting for the requester to connect'],
    ])('shows the %s without controls', async (_name, tool, canAnswer, expectedStatus) => {
        renderCard({tool, canAnswer});

        expect(screen.getByText(expectedStatus)).toBeTruthy();
        expect(screen.queryByRole('button')).toBeNull();
        await waitFor(() => expect(mockGetUserMCPTools).not.toHaveBeenCalled());
    });
});
