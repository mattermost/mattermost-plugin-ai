// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl, FormattedMessage} from 'react-intl';
import styled from 'styled-components';

import {Pill} from '../pill';

import {ItemList, SelectionItem, SelectionItemOption, TextItem} from './item';
import Panel from './panel';

interface UpstreamConfig {
    type: string;
    parameters: Record<string, unknown>;
}

export interface EmbeddingSearchConfig {
    type: string;
    vectorStore: UpstreamConfig;
    embeddingProvider: UpstreamConfig;
    parameters: Record<string, unknown>;
}

interface Props {
    value: EmbeddingSearchConfig;
    onChange: (config: EmbeddingSearchConfig) => void;
}

const Horizontal = styled.div`
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 8px;
`;

const EmbeddingSearchPanel = ({value, onChange}: Props) => {
    const intl = useIntl();

    return (
        <Panel
            title={
                <Horizontal>
                    <FormattedMessage defaultMessage='Embedding Search'/>
                    <Pill><FormattedMessage defaultMessage='EXPERIMENTAL'/></Pill>
                </Horizontal>
            }
            subtitle={intl.formatMessage({defaultMessage: 'Configure embedding search settings.'})}
        >
            <ItemList>
                <SelectionItem
                    label={intl.formatMessage({defaultMessage: 'Type'})}
                    value={value.type}
                    onChange={(e) => {
                        const newType = e.target.value;
                        if (newType === 'disabled') {
                            onChange({
                                type: 'disabled',
                                vectorStore: {type: '', parameters: {}},
                                embeddingProvider: {type: '', parameters: {}},
                                parameters: {},
                            });
                        } else if (value.type === 'disabled') {
                            // Set defaults when enabling
                            onChange({
                                type: newType,
                                vectorStore: {type: 'pgvector', parameters: {}},
                                embeddingProvider: {type: 'ollama', parameters: {model: '', apiURL: ''}},
                                parameters: {},
                            });
                        } else {
                            onChange({...value, type: newType});
                        }
                    }}
                >
                    <SelectionItemOption value='disabled'>{'Disabled'}</SelectionItemOption>
                    <SelectionItemOption value='composite'>{'Composite'}</SelectionItemOption>
                </SelectionItem>
                {value.type && value.type !== 'disabled' &&
                <SelectionItem
                    label={intl.formatMessage({defaultMessage: 'Vector Store Type'})}
                    value={value.vectorStore.type}
                    onChange={(e) => onChange({
                        ...value,
                        vectorStore: {...value.vectorStore, type: e.target.value},
                    })}
                >
                    <SelectionItemOption value='pgvector'>{'PostgreSQL pgvector'}</SelectionItemOption>
                </SelectionItem>
                }
                {value.type && value.type !== 'disabled' &&
                <SelectionItem
                    label={intl.formatMessage({defaultMessage: 'Embedding Provider Type'})}
                    value={value.embeddingProvider.type}
                    onChange={(e) => {
                        const newType = e.target.value;
                        const newParameters = newType === 'ollama' ? {model: '', apiURL: ''} : {};
                        onChange({
                            ...value,
                            embeddingProvider: {
                                type: newType,
                                parameters: newParameters,
                            },
                        });
                    }}
                >
                    <SelectionItemOption value='ollama'>{'Ollama'}</SelectionItemOption>
                </SelectionItem>
                }
                {value.type && value.type !== 'disabled' && value.embeddingProvider.type === 'ollama' && (
                    <>
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'Model'})}
                            value={(value.embeddingProvider.parameters?.model as string) || ''}
                            onChange={(e) => onChange({
                                ...value,
                                embeddingProvider: {
                                    ...value.embeddingProvider,
                                    parameters: {
                                        ...value.embeddingProvider.parameters,
                                        model: e.target.value,
                                    },
                                },
                            })}
                        />
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'API URL'})}
                            value={(value.embeddingProvider.parameters?.apiURL as string) || ''}
                            onChange={(e) => onChange({
                                ...value,
                                embeddingProvider: {
                                    ...value.embeddingProvider,
                                    parameters: {
                                        ...value.embeddingProvider.parameters,
                                        apiURL: e.target.value,
                                    },
                                },
                            })}
                        />
                    </>
                )}
            </ItemList>
        </Panel>
    );
};

export default EmbeddingSearchPanel;
