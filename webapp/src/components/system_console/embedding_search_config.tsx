// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl, FormattedMessage} from 'react-intl';
import styled from 'styled-components';

import {Pill} from '../pill';
import {PrimaryButton} from '../assets/buttons';
import ConfirmationDialog from '../confirmation_dialog';
import {doReindexPosts} from '../../client';

import {useIsBasicsLicensed} from '@/license';

import {ItemList, SelectionItem, SelectionItemOption, TextItem, HelpText, ItemLabel} from './item';
import Panel from './panel';
import EnterpriseChip from './enterprise_chip';

interface UpstreamConfig {
    type: string;
    parameters: Record<string, unknown>;
}

interface ChunkingOptions {
    chunkSize: number;
    chunkOverlap: number;
    minChunkSize: number;
    chunkingStrategy: string;
}

export interface EmbeddingSearchConfig {
    type: string;
    vectorStore: UpstreamConfig;
    embeddingProvider: UpstreamConfig;
    parameters: Record<string, unknown>;
    dimensions: number;
    chunkingOptions?: ChunkingOptions;
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

const ButtonContainer = styled.div`
    margin-top: 24px;
    padding-top: 24px;
    border-top: 1px solid rgba(var(--center-channel-color-rgb), 0.08);
    grid-column: 1 / -1;
`;

const ActionContainer = styled.div`
    display: grid;
    grid-template-columns: minmax(auto, 275px) 1fr;
    grid-column-gap: 16px;
`;

const SuccessHelpText = styled(HelpText)`
    margin-top: 8px;
    color: var(--online-indicator);
`;

const ErrorHelpText = styled(HelpText)`
    margin-top: 8px;
    color: var(--error-text);
`;

const EmbeddingSearchPanel = ({value, onChange}: Props) => {
    const intl = useIntl();
    const [showReindexConfirmation, setShowReindexConfirmation] = useState(false);
    const [isReindexing, setIsReindexing] = useState(false);
    const [reindexStatus, setReindexStatus] = useState<{success?: boolean; message?: string}>({});
    const isBasicsLicensed = useIsBasicsLicensed();

    const handleReindexClick = () => {
        setShowReindexConfirmation(true);
    };

    const handleConfirmReindex = async () => {
        setShowReindexConfirmation(false);
        setIsReindexing(true);
        setReindexStatus({});

        try {
            await doReindexPosts();
            setReindexStatus({
                success: true,
                message: intl.formatMessage({defaultMessage: 'Posts reindexing completed successfully.'}),
            });
        } catch (error) {
            setReindexStatus({
                success: false,
                message: intl.formatMessage({defaultMessage: 'Failed to reindex posts. Please try again.'}),
            });
        } finally {
            setIsReindexing(false);
        }
    };

    const handleCancelReindex = () => {
        setShowReindexConfirmation(false);
    };

    if (!isBasicsLicensed) {
        return (
            <Panel
                title={
                    <Horizontal>
                        <FormattedMessage defaultMessage='Embedding Search'/>
                        <Pill><FormattedMessage defaultMessage='EXPERIMENTAL'/></Pill>
                    </Horizontal>
                }
                subtitle={''}
            >
                <EnterpriseChip
                    text={intl.formatMessage({defaultMessage: 'Embeddings search is available on Enterprise plans'})}
                    subtext={intl.formatMessage({defaultMessage: 'Embeddings search is available on Enterprise plans'})}
                />
            </Panel>
        );
    }

    return (
        <Panel
            title={
                <Horizontal>
                    <FormattedMessage defaultMessage='Embedding Search'/>
                    <Pill><FormattedMessage defaultMessage='EXPERIMENTAL'/></Pill>
                </Horizontal>
            }
            subtitle={intl.formatMessage({defaultMessage: 'Configure embedding search settings. Note: The current implementation is experimental and subject to breaking changes. This includes having to reindex all posts.'})}
        >
            <ItemList>
                <SelectionItem
                    label={intl.formatMessage({defaultMessage: 'Type'})}
                    value={value.type}
                    onChange={(e) => {
                        const newType = e.target.value;
                        if (newType === '') {
                            onChange({
                                type: '',
                                vectorStore: {type: '', parameters: {}},
                                embeddingProvider: {type: '', parameters: {}},
                                parameters: {},
                                dimensions: 0,
                                chunkingOptions: {
                                    chunkSize: 1000,
                                    chunkOverlap: 200,
                                    minChunkSize: 0.75,
                                    chunkingStrategy: 'sentences',
                                },
                            });
                        } else if (value.type === '') {
                            // Set defaults when enabling
                            onChange({
                                type: newType,
                                vectorStore: {type: 'pgvector', parameters: {}},
                                embeddingProvider: {type: 'openai', parameters: {embeddingModel: '', apiKey: ''}},
                                parameters: {},
                                dimensions: 0,
                                chunkingOptions: {
                                    chunkSize: 1000,
                                    chunkOverlap: 200,
                                    minChunkSize: 0.75,
                                    chunkingStrategy: 'sentences',
                                },
                            });
                        } else {
                            onChange({...value, type: newType});
                        }
                    }}
                >
                    <SelectionItemOption value=''>{'Disabled'}</SelectionItemOption>
                    <SelectionItemOption value='composite'>{'Composite'}</SelectionItemOption>
                </SelectionItem>
                {value.type && value.type !== '' &&
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
                {value.type && value.type !== '' &&
                <SelectionItem
                    label={intl.formatMessage({defaultMessage: 'Embedding Provider Type'})}
                    value={value.embeddingProvider.type}
                    onChange={(e) => {
                        const newType = e.target.value;
                        let newParameters = {};
                        if (newType === 'openai-compatible') {
                            newParameters = {embeddingModel: '', apiKey: '', apiURL: ''};
                        } else if (newType === 'openai') {
                            newParameters = {embeddingModel: '', apiKey: ''};
                        }
                        onChange({
                            ...value,
                            embeddingProvider: {
                                type: newType,
                                parameters: newParameters,
                            },
                        });
                    }}
                >
                    <SelectionItemOption value='openai'>{'OpenAI'}</SelectionItemOption>
                    <SelectionItemOption value='openai-compatible'>{'OpenAI-compatible API'}</SelectionItemOption>
                </SelectionItem>
                }
                {value.type && value.type !== '' && value.embeddingProvider.type === 'openai' && (
                    <>
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'API Key'})}
                            type='password'
                            value={(value.embeddingProvider.parameters?.apiKey as string) || ''}
                            onChange={(e) => onChange({
                                ...value,
                                embeddingProvider: {
                                    ...value.embeddingProvider,
                                    parameters: {
                                        ...value.embeddingProvider.parameters,
                                        apiKey: e.target.value,
                                    },
                                },
                            })}
                        />
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'Model'})}
                            placeholder='Model'
                            value={(value.embeddingProvider.parameters?.embeddingModel as string) || ''}
                            onChange={(e) => onChange({
                                ...value,
                                embeddingProvider: {
                                    ...value.embeddingProvider,
                                    parameters: {
                                        ...value.embeddingProvider.parameters,
                                        embeddingModel: e.target.value,
                                    },
                                },
                            })}
                        />
                    </>
                )}

                {value.type && value.type !== '' && value.embeddingProvider.type === 'openai-compatible' && (
                    <>
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'API Key'})}
                            type='password'
                            value={(value.embeddingProvider.parameters?.apiKey as string) || ''}
                            onChange={(e) => onChange({
                                ...value,
                                embeddingProvider: {
                                    ...value.embeddingProvider,
                                    parameters: {
                                        ...value.embeddingProvider.parameters,
                                        apiKey: e.target.value,
                                    },
                                },
                            })}
                        />
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'Model'})}
                            value={(value.embeddingProvider.parameters?.embeddingModel as string) || ''}
                            onChange={(e) => onChange({
                                ...value,
                                embeddingProvider: {
                                    ...value.embeddingProvider,
                                    parameters: {
                                        ...value.embeddingProvider.parameters,
                                        embeddingModel: e.target.value,
                                    },
                                },
                            })}
                        />
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'API URL'})}
                            placeholder='http://localhost:11434/v1'
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

                {value.type === 'composite' && (
                    <>
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'Dimensions'})}
                            type='number'
                            placeholder='1024'
                            value={value?.dimensions?.toString() || '0'}
                            onChange={(e) => {
                                const rawParse = parseInt(e.target.value, 10);
                                const dimensionsValue = isNaN(rawParse) ? 0 : rawParse;
                                onChange({
                                    ...value,
                                    dimensions: dimensionsValue,
                                });
                            }}
                            helptext={intl.formatMessage({defaultMessage: 'The number of dimensions for the vector embeddings. Common values are 768, 1024, or 1536 depending on the model.'})}
                        />

                        {/* Chunking Options Section */}
                        {/* Define default chunking options in one place to maintain consistency */}
                        {(() => {
                            const defaultChunkingOptions = {
                                chunkSize: 1000,
                                chunkOverlap: 200,
                                minChunkSize: 0.75,
                                chunkingStrategy: 'sentences',
                            };

                            return (
                                <>
                                    <SelectionItem
                                        label={intl.formatMessage({defaultMessage: 'Chunking Strategy'})}
                                        value={value.chunkingOptions?.chunkingStrategy || defaultChunkingOptions.chunkingStrategy}
                                        onChange={(e) => onChange({
                                            ...value,
                                            chunkingOptions: {
                                                ...(value.chunkingOptions || defaultChunkingOptions),
                                                chunkingStrategy: e.target.value,
                                            } as ChunkingOptions,
                                        })}
                                        helptext={intl.formatMessage({defaultMessage: 'The strategy to use for splitting text into chunks.'})}
                                    >
                                        <SelectionItemOption value='sentences'>{'Sentences'}</SelectionItemOption>
                                        <SelectionItemOption value='paragraphs'>{'Paragraphs'}</SelectionItemOption>
                                        <SelectionItemOption value='fixed'>{'Fixed Size'}</SelectionItemOption>
                                    </SelectionItem>

                                    <TextItem
                                        label={intl.formatMessage({defaultMessage: 'Chunk Size'})}
                                        type='number'
                                        placeholder={defaultChunkingOptions.chunkSize.toString()}
                                        value={(value.chunkingOptions?.chunkSize || defaultChunkingOptions.chunkSize).toString()}
                                        onChange={(e) => {
                                            const rawParse = parseInt(e.target.value, 10);
                                            const chunkSize = isNaN(rawParse) ? defaultChunkingOptions.chunkSize : rawParse;
                                            onChange({
                                                ...value,
                                                chunkingOptions: {
                                                    ...(value.chunkingOptions || defaultChunkingOptions),
                                                    chunkSize,
                                                } as ChunkingOptions,
                                            });
                                        }}
                                        helptext={intl.formatMessage({defaultMessage: 'Maximum size of each chunk in characters.'})}
                                    />

                                    <TextItem
                                        label={intl.formatMessage({defaultMessage: 'Chunk Overlap'})}
                                        type='number'
                                        placeholder={defaultChunkingOptions.chunkOverlap.toString()}
                                        value={(value.chunkingOptions?.chunkOverlap || defaultChunkingOptions.chunkOverlap).toString()}
                                        onChange={(e) => {
                                            const rawParse = parseInt(e.target.value, 10);
                                            const chunkOverlap = isNaN(rawParse) ? defaultChunkingOptions.chunkOverlap : rawParse;
                                            onChange({
                                                ...value,
                                                chunkingOptions: {
                                                    ...(value.chunkingOptions || defaultChunkingOptions),
                                                    chunkOverlap,
                                                } as ChunkingOptions,
                                            });
                                        }}
                                        helptext={intl.formatMessage({defaultMessage: 'Number of characters to overlap between chunks (only used for fixed size chunking).'})}
                                    />

                                    <TextItem
                                        label={intl.formatMessage({defaultMessage: 'Minimum Chunk Size Ratio'})}
                                        type='number'
                                        step='0.01'
                                        min='0'
                                        max='1'
                                        placeholder={defaultChunkingOptions.minChunkSize.toString()}
                                        value={(value.chunkingOptions?.minChunkSize || defaultChunkingOptions.minChunkSize).toString()}
                                        onChange={(e) => {
                                            const rawParse = parseFloat(e.target.value);
                                            const minChunkSize = isNaN(rawParse) ? defaultChunkingOptions.minChunkSize : Math.min(Math.max(rawParse, 0), 1);
                                            onChange({
                                                ...value,
                                                chunkingOptions: {
                                                    ...(value.chunkingOptions || defaultChunkingOptions),
                                                    minChunkSize,
                                                } as ChunkingOptions,
                                            });
                                        }}
                                        helptext={intl.formatMessage({defaultMessage: 'Minimum chunk size as a fraction of the maximum size (0.0-1.0). Used for sentence and paragraph chunking.'})}
                                    />
                                </>
                            );
                        })()}
                    </>
                )}

                {value.type && value.type !== '' && (
                    <ButtonContainer>
                        <ActionContainer>
                            <ItemLabel>
                                <FormattedMessage defaultMessage='Reindex All Posts'/>
                            </ItemLabel>
                            <div>
                                <PrimaryButton
                                    onClick={handleReindexClick}
                                    disabled={isReindexing}
                                >
                                    {isReindexing ? <FormattedMessage defaultMessage='Reindexing...'/> : <FormattedMessage defaultMessage='Reindex Posts'/>
                                    }
                                </PrimaryButton>

                                {reindexStatus.message && (
                                    reindexStatus.success ? (
                                        <SuccessHelpText>
                                            {reindexStatus.message}
                                        </SuccessHelpText>
                                    ) : (
                                        <ErrorHelpText>
                                            {reindexStatus.message}
                                        </ErrorHelpText>
                                    )
                                )}

                                <HelpText>
                                    <FormattedMessage defaultMessage='Reindex all posts to update the embedding search database. This process will clear the current index and rebuild it from scratch. It may take a significant amount of time for large installations.'/>
                                </HelpText>
                            </div>
                        </ActionContainer>
                    </ButtonContainer>
                )}
            </ItemList>

            {showReindexConfirmation && (
                <ConfirmationDialog
                    title={<FormattedMessage defaultMessage='Confirm Reindexing'/>}
                    message={
                        <>
                            <p>
                                <FormattedMessage defaultMessage='Are you sure you want to reindex all posts?'/>
                            </p>
                            <p>
                                <FormattedMessage defaultMessage='This will clear the current index and rebuild it from scratch. The process will:'/>
                            </p>
                            <ul>
                                <li><FormattedMessage defaultMessage='Index all existing posts in the database'/></li>
                                <li><FormattedMessage defaultMessage='Take a significant amount of time for large installations'/></li>
                                <li><FormattedMessage defaultMessage='Increase database load during the reindexing process'/></li>
                            </ul>
                        </>
                    }
                    confirmButtonText={<FormattedMessage defaultMessage='Reindex'/>}
                    onConfirm={handleConfirmReindex}
                    onCancel={handleCancelReindex}
                />
            )}
        </Panel>
    );
};

export default EmbeddingSearchPanel;
