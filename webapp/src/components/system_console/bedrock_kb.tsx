// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import styled from 'styled-components';

import {TrashCanOutlineIcon, PlusIcon} from '@mattermost/compass-icons/components';

import {ButtonIcon} from '../assets/buttons';

import {TextItem, BooleanItem} from './item';

export type KnowledgeBaseConfig = {
    id: string;
    name: string;
    description: string;
    maxResults: number;
    relevanceThreshold: number;
    enabled: boolean;
}

export type BedrockKnowledgeBaseSettings = {
    bedrockKBRegion: string;
    bedrockKBAPIKey: string;
    bedrockKBAPISecret: string;
    bedrockKnowledgeBases: KnowledgeBaseConfig[];
}

const KnowledgeBaseItem = styled.div`
    margin-bottom: 16px;
    padding: 12px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.08);
    border-radius: 4px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
`;

const KnowledgeBaseHeader = styled.div`
    display: flex;
    align-items: center;
    margin-bottom: 16px;
`;

const KnowledgeBaseTitle = styled.div`
    font-weight: 600;
    margin-right: auto;
`;

const EmptyState = styled.div`
    text-align: center;
    padding: 30px 0;
    color: rgba(var(--center-channel-color-rgb), 0.6);
`;

const AddKnowledgeBaseButton = styled.button`
    display: flex;
    align-items: center;
    justify-content: center;
    height: 36px;
    padding: 0 16px;
    background: none;
    border-radius: 4px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.24);
    color: var(--center-channel-color);
    font-weight: 600;
    margin: 12px 0;
    cursor: pointer;
    
    &:hover {
        background: rgba(var(--center-channel-color-rgb), 0.08);
    }
    
    span {
        margin-left: 8px;
    }
`;

type Props = {
    config: BedrockKnowledgeBaseSettings;
    onChange: (config: BedrockKnowledgeBaseSettings) => void;
}

export const BedrockKnowledgeBaseSettingsComponent = ({config, onChange}: Props) => {
    const handleAddKnowledgeBase = () => {
        // Create a deep copy of the config object
        const newConfig = {
            ...config,
            bedrockKnowledgeBases: config.bedrockKnowledgeBases ? [...config.bedrockKnowledgeBases] : []
        };
        
        const newKB: KnowledgeBaseConfig = {
            id: `kb-${Date.now()}`, // Generate a unique ID
            name: '',
            description: '',
            maxResults: 5,
            relevanceThreshold: 0.5,
            enabled: true,
        };

        newConfig.bedrockKnowledgeBases.push(newKB);
        onChange(newConfig);
    };

    const handleRemoveKnowledgeBase = (index: number) => {
        // Create a deep copy of the config object
        const newConfig = {
            ...config,
            bedrockKnowledgeBases: config.bedrockKnowledgeBases ? [...config.bedrockKnowledgeBases] : []
        };
        
        newConfig.bedrockKnowledgeBases.splice(index, 1);
        onChange(newConfig);
    };

    const handleKnowledgeBaseChange = (index: number, updatedKB: KnowledgeBaseConfig) => {
        // Create a deep copy of the config object
        const newConfig = {
            ...config,
            bedrockKnowledgeBases: config.bedrockKnowledgeBases ? [...config.bedrockKnowledgeBases] : []
        };
        
        newConfig.bedrockKnowledgeBases[index] = updatedKB;
        onChange(newConfig);
    };

    return (
        <div className='wrapper--fixed'>
            <div className='admin-console__header'>
                <FormattedMessage
                    id='admin.plugin.ai.bedrock_kb.title'
                    defaultMessage='AWS Bedrock Knowledge Bases'
                />
                <div className='admin-console__header-description'>
                    <FormattedMessage
                        id='admin.plugin.ai.bedrock_kb.description'
                        defaultMessage='Configure AWS Bedrock Knowledge Bases to enable searches within your company knowledge repositories.'
                    />
                </div>
            </div>

            <form className='form-horizontal'>
                <div className='form-group'>
                    <label className='control-label col-sm-4'>
                        <FormattedMessage
                            id='admin.plugin.ai.bedrock_kb.region'
                            defaultMessage='AWS Region'
                        />
                    </label>
                    <div className='col-sm-8'>
                        <TextItem
                            label="AWS Region"
                            value={config.bedrockKBRegion || ''}
                            onChange={(e) => onChange({...config, bedrockKBRegion: e.target.value})}
                            placeholder='e.g., us-west-2'
                        />
                        <div className='help-text'>
                            <FormattedMessage
                                id='admin.plugin.ai.bedrock_kb.region.help'
                                defaultMessage='The AWS region where your Bedrock Knowledge Bases are hosted.'
                            />
                        </div>
                    </div>
                </div>

                <div className='form-group'>
                    <label className='control-label col-sm-4'>
                        <FormattedMessage
                            id='admin.plugin.ai.bedrock_kb.apiKey'
                            defaultMessage='AWS API Key'
                        />
                    </label>
                    <div className='col-sm-8'>
                        <TextItem
                            label="AWS API Key"
                            type='password'
                            value={config.bedrockKBAPIKey || ''}
                            onChange={(e) => onChange({...config, bedrockKBAPIKey: e.target.value})}
                            placeholder='Your AWS API Key'
                        />
                        <div className='help-text'>
                            <FormattedMessage
                                id='admin.plugin.ai.bedrock_kb.apiKey.help'
                                defaultMessage='The AWS API Key for accessing Bedrock Knowledge Bases.'
                            />
                        </div>
                    </div>
                </div>

                <div className='form-group'>
                    <label className='control-label col-sm-4'>
                        <FormattedMessage
                            id='admin.plugin.ai.bedrock_kb.apiSecret'
                            defaultMessage='AWS API Secret'
                        />
                    </label>
                    <div className='col-sm-8'>
                        <TextItem
                            label="AWS API Secret"
                            type='password'
                            value={config.bedrockKBAPISecret || ''}
                            onChange={(e) => onChange({...config, bedrockKBAPISecret: e.target.value})}
                            placeholder='Your AWS API Secret'
                        />
                        <div className='help-text'>
                            <FormattedMessage
                                id='admin.plugin.ai.bedrock_kb.apiSecret.help'
                                defaultMessage='The AWS API Secret for accessing Bedrock Knowledge Bases.'
                            />
                        </div>
                    </div>
                </div>

                <div className='form-group'>
                    <label className='control-label col-sm-4'>
                        <FormattedMessage
                            id='admin.plugin.ai.bedrock_kb.knowledgeBases'
                            defaultMessage='Knowledge Bases'
                        />
                    </label>
                    <div className='col-sm-8'>
                        {(!config.bedrockKnowledgeBases || config.bedrockKnowledgeBases.length === 0) && (
                            <EmptyState>
                                <FormattedMessage
                                    id='admin.plugin.ai.bedrock_kb.noKnowledgeBases'
                                    defaultMessage='No knowledge bases configured. Add a knowledge base to enable knowledge search capabilities.'
                                />
                            </EmptyState>
                        )}

                        {config.bedrockKnowledgeBases && config.bedrockKnowledgeBases.map((kb, index) => (
                            <KnowledgeBaseItem key={kb.id || index}>
                                <KnowledgeBaseHeader>
                                    <KnowledgeBaseTitle>
                                        {kb.name || (
                                            <FormattedMessage
                                                id='admin.plugin.ai.bedrock_kb.unnamed'
                                                defaultMessage='Unnamed Knowledge Base'
                                            />
                                        )}
                                    </KnowledgeBaseTitle>
                                    <ButtonIcon
                                        onClick={() => handleRemoveKnowledgeBase(index)}
                                    >
                                        <TrashCanOutlineIcon size={18}/>
                                    </ButtonIcon>
                                </KnowledgeBaseHeader>

                                <div className='form-group'>
                                    <label className='control-label col-sm-4'>
                                        <FormattedMessage
                                            id='admin.plugin.ai.bedrock_kb.id'
                                            defaultMessage='Knowledge Base ID'
                                        />
                                    </label>
                                    <div className='col-sm-8'>
                                        <TextItem
                                            label="Knowledge Base ID"
                                            value={kb.id || ''}
                                            onChange={(e) => handleKnowledgeBaseChange(index, {...kb, id: e.target.value})}
                                            placeholder='e.g., kbid-123456abcdef'
                                        />
                                    </div>
                                </div>

                                <div className='form-group'>
                                    <label className='control-label col-sm-4'>
                                        <FormattedMessage
                                            id='admin.plugin.ai.bedrock_kb.name'
                                            defaultMessage='Display Name'
                                        />
                                    </label>
                                    <div className='col-sm-8'>
                                        <TextItem
                                            label="Display Name"
                                            value={kb.name || ''}
                                            onChange={(e) => handleKnowledgeBaseChange(index, {...kb, name: e.target.value})}
                                            placeholder='e.g., Company Documentation'
                                        />
                                    </div>
                                </div>

                                <div className='form-group'>
                                    <label className='control-label col-sm-4'>
                                        <FormattedMessage
                                            id='admin.plugin.ai.bedrock_kb.description'
                                            defaultMessage='Description'
                                        />
                                    </label>
                                    <div className='col-sm-8'>
                                        <TextItem
                                            label="Description"
                                            value={kb.description || ''}
                                            onChange={(e) => handleKnowledgeBaseChange(index, {...kb, description: e.target.value})}
                                            placeholder='e.g., Contains company policies and procedures'
                                        />
                                    </div>
                                </div>

                                <div className='form-group'>
                                    <label className='control-label col-sm-4'>
                                        <FormattedMessage
                                            id='admin.plugin.ai.bedrock_kb.maxResults'
                                            defaultMessage='Max Results'
                                        />
                                    </label>
                                    <div className='col-sm-8'>
                                        <TextItem
                                            label="Max Results"
                                            type='number'
                                            value={kb.maxResults?.toString() || '5'}
                                            onChange={(e) => handleKnowledgeBaseChange(index, {...kb, maxResults: parseInt(e.target.value, 10) || 5})}
                                            placeholder='5'
                                        />
                                        <div className='help-text'>
                                            <FormattedMessage
                                                id='admin.plugin.ai.bedrock_kb.maxResults.help'
                                                defaultMessage='Maximum number of results to return per query (1-20).'
                                            />
                                        </div>
                                    </div>
                                </div>

                                <div className='form-group'>
                                    <label className='control-label col-sm-4'>
                                        <FormattedMessage
                                            id='admin.plugin.ai.bedrock_kb.relevanceThreshold'
                                            defaultMessage='Relevance Threshold'
                                        />
                                    </label>
                                    <div className='col-sm-8'>
                                        <TextItem
                                            label="Relevance Threshold"
                                            type='number'
                                            value={kb.relevanceThreshold?.toString() || '0.5'}
                                            onChange={(e) => handleKnowledgeBaseChange(index, {...kb, relevanceThreshold: parseFloat(e.target.value) || 0.5})}
                                            placeholder='0.5'
                                        />
                                        <div className='help-text'>
                                            <FormattedMessage
                                                id='admin.plugin.ai.bedrock_kb.relevanceThreshold.help'
                                                defaultMessage='Minimum relevance score for results (0.0-1.0).'
                                            />
                                        </div>
                                    </div>
                                </div>

                                <div className='form-group'>
                                    <label className='control-label col-sm-4'>
                                        <FormattedMessage
                                            id='admin.plugin.ai.bedrock_kb.enabled'
                                            defaultMessage='Enabled'
                                        />
                                    </label>
                                    <div className='col-sm-8'>
                                        <BooleanItem
                                            label="Enabled"
                                            value={kb.enabled ?? true}
                                            onChange={(value) => handleKnowledgeBaseChange(index, {...kb, enabled: value})}
                                        />
                                    </div>
                                </div>
                            </KnowledgeBaseItem>
                        ))}

                        <AddKnowledgeBaseButton
                            onClick={handleAddKnowledgeBase}
                            type='button'
                        >
                            <PlusIcon size={16}/>
                            <span>
                                <FormattedMessage
                                    id='admin.plugin.ai.bedrock_kb.addKnowledgeBase'
                                    defaultMessage='Add Knowledge Base'
                                />
                            </span>
                        </AddKnowledgeBaseButton>
                    </div>
                </div>
            </form>
        </div>
    );
};

export default BedrockKnowledgeBaseSettingsComponent;