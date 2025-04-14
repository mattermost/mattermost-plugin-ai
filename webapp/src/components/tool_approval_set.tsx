// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import styled from 'styled-components';

import {doToolCall} from '@/client';

import {ToolCall, ToolCallStatus} from './llmbot_post';
import ToolCard from './tool_card';

// Styled components
const ToolCallsContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin: 16px 0;
`;

// Tool call interfaces
interface ToolApprovalSetProps {
    postID: string;
    toolCalls: ToolCall[];
    onApprove: (toolID: string) => Promise<void>;
    onReject: (toolID: string) => Promise<void>;
}

const ToolApprovalSet: React.FC<ToolApprovalSetProps> = (props) => {
    // Track which tools are currently being processed
    const [processingTools, setProcessingTools] = useState<string[]>([]);
    const [error, setError] = useState('');
    const [collapsedTools, setCollapsedTools] = useState<string[]>([]);

    const handleApprove = async (toolID: string) => {
        try {
            // Update UI immediately to prevent double-clicking
            setProcessingTools((prev) => [...prev, toolID]);

            // Call the API endpoint
            await doToolCall(props.postID, [toolID]);

            // Also call the onApprove callback for backward compatibility
            if (props.onApprove) {
                await props.onApprove(toolID);
            }
        } catch (err) {
            setError('Failed to approve tool call');

            // Remove from processing state on error
            setProcessingTools((prev) => prev.filter((id) => id !== toolID));
        }
    };

    const handleReject = async (toolID: string) => {
        try {
            // Update UI immediately to prevent double-clicking
            setProcessingTools((prev) => [...prev, toolID]);

            // Call the API endpoint
            await doToolCall(props.postID, []);

            // Also call the onReject callback for backward compatibility
            if (props.onReject) {
                await props.onReject(toolID);
            }
        } catch (err) {
            setError('Failed to reject tool call');

            // Remove from processing state on error
            setProcessingTools((prev) => prev.filter((id) => id !== toolID));
        }
    };

    const toggleCollapse = (toolID: string) => {
        setCollapsedTools((prev) =>
            (prev.includes(toolID) ?
                prev.filter((id) => id !== toolID) :
                [...prev, toolID]),
        );
    };

    if (props.toolCalls.length === 0) {
        return null;
    }

    if (error) {
        return <div className='error'>{error}</div>;
    }

    // Get pending tool calls
    const pendingToolCalls = props.toolCalls.filter((call) => call.status === ToolCallStatus.Pending);

    // Get processed tool calls
    const processedToolCalls = props.toolCalls.filter((call) => call.status !== ToolCallStatus.Pending);

    return (
        <ToolCallsContainer>
            {pendingToolCalls.map((tool) => (
                <ToolCard
                    key={tool.id}
                    tool={tool}
                    isCollapsed={collapsedTools.includes(tool.id)}
                    isProcessing={processingTools.includes(tool.id)}
                    onToggleCollapse={() => toggleCollapse(tool.id)}
                    onApprove={() => handleApprove(tool.id)}
                    onReject={() => handleReject(tool.id)}
                />
            ))}

            {processedToolCalls.map((tool) => (
                <ToolCard
                    key={tool.id}
                    tool={tool}
                    isCollapsed={collapsedTools.includes(tool.id)}
                    isProcessing={false}
                    onToggleCollapse={() => toggleCollapse(tool.id)}
                />
            ))}
        </ToolCallsContainer>
    );
};

export default ToolApprovalSet;
