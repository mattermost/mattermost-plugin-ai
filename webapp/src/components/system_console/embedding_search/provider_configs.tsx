// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {TextItem} from '../item';

import {UpstreamConfig} from './types';

interface OpenAIConfigProps {
    value: UpstreamConfig;
    onChange: (config: UpstreamConfig) => void;
}

export const OpenAIProviderConfig = ({value, onChange}: OpenAIConfigProps) => {
    const intl = useIntl();

    return (
        <>
            <TextItem
                label={intl.formatMessage({defaultMessage: 'API Key'})}
                type='password'
                value={(value.parameters?.apiKey as string) || ''}
                onChange={(e) => onChange({
                    ...value,
                    parameters: {
                        ...value.parameters,
                        apiKey: e.target.value,
                    },
                })}
            />
            <TextItem
                label={intl.formatMessage({defaultMessage: 'Model'})}
                placeholder='Model'
                value={(value.parameters?.embeddingModel as string) || ''}
                onChange={(e) => onChange({
                    ...value,
                    parameters: {
                        ...value.parameters,
                        embeddingModel: e.target.value,
                    },
                })}
            />
        </>
    );
};

export const OpenAICompatibleProviderConfig = ({value, onChange}: OpenAIConfigProps) => {
    const intl = useIntl();

    return (
        <>
            <TextItem
                label={intl.formatMessage({defaultMessage: 'API Key'})}
                type='password'
                value={(value.parameters?.apiKey as string) || ''}
                onChange={(e) => onChange({
                    ...value,
                    parameters: {
                        ...value.parameters,
                        apiKey: e.target.value,
                    },
                })}
            />
            <TextItem
                label={intl.formatMessage({defaultMessage: 'Model'})}
                value={(value.parameters?.embeddingModel as string) || ''}
                onChange={(e) => onChange({
                    ...value,
                    parameters: {
                        ...value.parameters,
                        embeddingModel: e.target.value,
                    },
                })}
            />
            <TextItem
                label={intl.formatMessage({defaultMessage: 'API URL'})}
                placeholder='http://localhost:11434/v1'
                value={(value.parameters?.apiURL as string) || ''}
                onChange={(e) => onChange({
                    ...value,
                    parameters: {
                        ...value.parameters,
                        apiURL: e.target.value,
                    },
                })}
            />
        </>
    );
};