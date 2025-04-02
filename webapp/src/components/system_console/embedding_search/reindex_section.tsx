// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import styled from 'styled-components';

import {PrimaryButton, SecondaryButton} from '../../assets/buttons';

import {HelpText, ItemLabel} from '../item';

import {JobStatusType, StatusMessageType} from './types';

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

const ProgressContainer = styled.div`
    margin-top: 8px;
    width: 100%;
    background-color: rgba(var(--center-channel-color-rgb), 0.08);
    border-radius: 4px;
    height: 8px;
    overflow: hidden;
`;

const ProgressBar = styled.div<{progress: number}>`
    height: 100%;
    width: ${(props) => props.progress}%;
    background-color: var(--button-bg);
    transition: width 0.3s ease-in-out;
`;

const ProgressText = styled(HelpText)`
    margin-top: 8px;
    margin-bottom: 12px;
    font-size: 12px;
`;

const ButtonGroup = styled.div`
    display: flex;
    gap: 8px;
`;

interface ReindexSectionProps {
    jobStatus: JobStatusType | null;
    statusMessage: StatusMessageType;
    onReindexClick: () => void;
    onCancelJob: () => void;
}

export const ReindexSection = ({
    jobStatus,
    statusMessage,
    onReindexClick,
    onCancelJob,
}: ReindexSectionProps) => {
    // Check if job is running
    const isReindexing = jobStatus?.status === 'running';

    return (
        <ButtonContainer>
            <ActionContainer>
                <ItemLabel>
                    <FormattedMessage defaultMessage='Reindex All Posts'/>
                </ItemLabel>
                <div>
                    {/* Show different UI based on job status */}
                    {isReindexing ? (
                        <>
                            <ButtonGroup>
                                <SecondaryButton onClick={onCancelJob}>
                                    <FormattedMessage defaultMessage='Cancel Reindexing'/>
                                </SecondaryButton>
                            </ButtonGroup>

                            {jobStatus && (
                                <>
                                    <ProgressText>
                                        <FormattedMessage
                                            defaultMessage='Processing: {processed} of {total} posts ({percent}%)'
                                            values={{
                                                processed: jobStatus.processed_rows.toLocaleString(),
                                                total: jobStatus.total_rows.toLocaleString(),
                                                percent: jobStatus.total_rows ? Math.floor((jobStatus.processed_rows / jobStatus.total_rows) * 100) : 0,
                                            }}
                                        />
                                    </ProgressText>
                                    <ProgressContainer>
                                        <ProgressBar
                                            progress={jobStatus.total_rows ? Math.min((jobStatus.processed_rows / jobStatus.total_rows) * 100, 100) : 0}
                                        />
                                    </ProgressContainer>
                                </>
                            )}
                        </>
                    ) : (
                        <PrimaryButton onClick={onReindexClick}>
                            <FormattedMessage defaultMessage='Reindex Posts'/>
                        </PrimaryButton>
                    )}

                    {statusMessage.message && (
                        statusMessage.success ? (
                            <SuccessHelpText>
                                {statusMessage.message}
                            </SuccessHelpText>
                        ) : (
                            <ErrorHelpText>
                                {statusMessage.message}
                            </ErrorHelpText>
                        )
                    )}

                    <HelpText>
                        <FormattedMessage defaultMessage='Reindex all posts to update the embedding search database. This process will clear the current index and rebuild it from scratch. It may take a significant amount of time for large installations.'/>
                    </HelpText>
                </div>
            </ActionContainer>
        </ButtonContainer>
    );
};
