import React from 'react';
import styled from 'styled-components';

const ActionContainer = styled.div`
    margin-bottom: 12px;
    padding: 12px;
    background: var(--center-channel-bg);
    border-radius: 4px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.08);

    &:last-child {
        margin-bottom: 0;
    }
`;

const ActionHeader = styled.div`
    display: flex;
    align-items: center;
    margin-bottom: 8px;
`;

const ActionName = styled.div`
    font-weight: 600;
    color: var(--center-channel-color);
    margin-right: 8px;
`;

const ActionDescription = styled.div`
    color: rgba(var(--center-channel-color-rgb), 0.64);
    font-size: 12px;
`;

const FieldsContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: 8px;
`;

const FieldGroup = styled.div`
    display: flex;
    flex-direction: column;
    gap: 4px;
`;

const FieldLabel = styled.div`
    font-size: 12px;
    color: rgba(var(--center-channel-color-rgb), 0.64);
`;

const FieldValue = styled.div`
    padding: 6px 8px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
    border-radius: 4px;
    font-family: monospace;
    font-size: 13px;
`;

interface MicroactionDisplayProps {
    action: {
        action: string;
        payload: Record<string, any>;
    };
}

const getActionMetadata = (actionName: string) => {
    const metadata: Record<string, {
        description: string;
        required: string[];
        optional?: string[];
    }> = {
        create_channel: {
            description: 'Creates a new channel',
            required: ['team_id', 'name', 'display_name', 'type'],
            optional: ['purpose', 'header'],
        },
        add_channel_member: {
            description: 'Adds a user to a channel',
            required: ['channel_id', 'user_id'],
        },
        create_post: {
            description: 'Creates a new post',
            required: ['channel_id', 'message'],
            optional: ['root_id', 'file_ids', 'props'],
        },
        update_user_preferences: {
            description: 'Updates preferences for a user',
            required: ['user_id', 'preferences'],
        },
        execute_slash_command: {
            description: 'Executes a slash command',
            required: ['channel_id', 'command'],
            optional: ['team_id', 'root_id', 'parent_id'],
        },
    };

    return metadata[actionName] || {
        description: 'Unknown action',
        required: [],
    };
};

const MicroactionDisplay: React.FC<MicroactionDisplayProps> = ({action}) => {
    const metadata = getActionMetadata(action.action);
    const allFields = [...metadata.required, ...(metadata.optional || [])];

    return (
        <ActionContainer>
            <ActionHeader>
                <ActionName>{action.action}</ActionName>
                <ActionDescription>{metadata.description}</ActionDescription>
            </ActionHeader>
            <FieldsContainer>
                {allFields.map((field) => {
                    const value = action.payload[field];
                    if (value === undefined) {
                        return null;
                    }

                    return (
                        <FieldGroup key={field}>
                            <FieldLabel>
                                {field}
                                {metadata.required.includes(field) && ' *'}
                            </FieldLabel>
                            <FieldValue>
                                {typeof value === 'object' 
                                    ? JSON.stringify(value, null, 2)
                                    : String(value)
                                }
                            </FieldValue>
                        </FieldGroup>
                    );
                })}
            </FieldsContainer>
        </ActionContainer>
    );
};

export default MicroactionDisplay;
