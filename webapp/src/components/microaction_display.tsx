import React, {useState} from 'react';
import styled from 'styled-components';

const ActionContainer = styled.div`
    margin-bottom: 12px;
    background: var(--center-channel-bg);
    border-radius: 4px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.08);

    &:last-child {
        margin-bottom: 0;
    }
`;

const ActionHeader = styled.div<{isExpanded: boolean}>`
    display: flex;
    align-items: center;
    padding: 12px;
    cursor: pointer;
    border-bottom: ${props => props.isExpanded ? '1px solid rgba(var(--center-channel-color-rgb), 0.08)' : 'none'};

    &:hover {
        background: rgba(var(--center-channel-color-rgb), 0.04);
    }
`;

const ExpandIcon = styled.span<{isExpanded: boolean}>`
    margin-right: 8px;
    transform: ${props => props.isExpanded ? 'rotate(90deg)' : 'none'};
    transition: transform 0.15s ease-in-out;
`;

const PrimaryField = styled.div`
    color: rgba(var(--center-channel-color-rgb), 0.72);
    font-size: 13px;
    margin-left: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 300px;
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
        primaryField?: string;
    }> = {
        create_channel: {
            description: 'Creates a new channel',
            required: ['team_id', 'name', 'display_name', 'type'],
            optional: ['purpose', 'header'],
            primaryField: 'display_name',
        },
        add_channel_member: {
            description: 'Adds a user to a channel',
            required: ['channel_id', 'user_id'],
            primaryField: 'user_id',
        },
        create_post: {
            description: 'Creates a new post',
            required: ['channel_id', 'message'],
            optional: ['root_id', 'file_ids', 'props'],
            primaryField: 'message',
        },
        update_user_preferences: {
            description: 'Updates preferences for a user',
            required: ['user_id', 'preferences'],
            primaryField: 'user_id',
        },
        execute_slash_command: {
            description: 'Executes a slash command',
            required: ['channel_id', 'command'],
            optional: ['team_id', 'root_id', 'parent_id'],
            primaryField: 'command',
        },
        create_user: {
            description: 'Creates a new user',
            required: ['username', 'email', 'password'],
            optional: ['nickname', 'first_name', 'last_name', 'locale'],
            primaryField: 'username',
        },
        remove_channel_member: {
            description: 'Removes a user from a channel',
            required: ['channel_id', 'user_id'],
            primaryField: 'user_id',
        },
        create_team: {
            description: 'Creates a new team',
            required: ['name', 'display_name', 'type'],
            optional: ['description', 'allow_open_invite'],
            primaryField: 'display_name',
        },
        add_team_member: {
            description: 'Adds a user to a team',
            required: ['team_id', 'user_id'],
            primaryField: 'user_id',
        },
        remove_team_member: {
            description: 'Removes a user from a team',
            required: ['team_id', 'user_id', 'requestor_id'],
            primaryField: 'user_id',
        },
        update_channel: {
            description: 'Updates an existing channel',
            required: ['id', 'name', 'display_name', 'type'],
            optional: ['purpose', 'header'],
            primaryField: 'display_name',
        },
    };

    return metadata[actionName] || {
        description: 'Unknown action',
        required: [],
    };
};

const MicroactionDisplay: React.FC<MicroactionDisplayProps> = ({action}) => {
    const [isExpanded, setIsExpanded] = useState(false);
    const metadata = getActionMetadata(action.action);
    const allFields = [...metadata.required, ...(metadata.optional || [])];
    const primaryValue = metadata.primaryField ? action.payload[metadata.primaryField] : null;

    return (
        <ActionContainer>
            <ActionHeader
                onClick={() => setIsExpanded(!isExpanded)}
                isExpanded={isExpanded}
            >
                <ExpandIcon isExpanded={isExpanded}>▶</ExpandIcon>
                <ActionName>{action.action}</ActionName>
                <ActionDescription>{metadata.description}</ActionDescription>
                {primaryValue && (
                    <PrimaryField>
                        {typeof primaryValue === 'object'
                            ? JSON.stringify(primaryValue)
                            : String(primaryValue)
                        }
                    </PrimaryField>
                )}
            </ActionHeader>
            {isExpanded && (
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
            )}
        </ActionContainer>
    );
};

export default MicroactionDisplay;
