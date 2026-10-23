// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {FormattedMessage} from 'react-intl';
import styled from 'styled-components';

import {useChannelAutoReplyDraft} from './autoreply_state';

type Props = {
    informChange: (name: string, value: string) => void;
};

type AmbientFieldName = 'instructions' | 'analysis_model';

// Ambient extras for the schema-driven channel-settings tab. The host hands
// custom settings nothing but informChange, so saved values come from the
// module-level draft seeded by loadValues.
//
// Known host limitation (documented, do not work around): SaveChangesPanel
// Reset restores host-owned values only — custom components get no signal —
// so after a Reset these fields keep displaying local text while the host
// reverted to baseline. Saving then persists baseline extras; reopening
// re-hydrates. Calling informChange from an effect to compensate would make
// the tab instantly dirty on open.
const AmbientField = ({informChange, name, label, helpText, multiline}: Props & {
    name: AmbientFieldName;
    label: React.ReactNode;
    helpText: React.ReactNode;
    multiline?: boolean;
}) => {
    const draft = useChannelAutoReplyDraft();
    const [local, setLocal] = useState<string | null>(null);

    if (!draft) {
        return null;
    }

    // Static ids are safe: only one Channel Settings modal exists at a time.
    const id = `autoreply-${name.replace('_', '-')}`;
    const onChange = (event: React.ChangeEvent<HTMLTextAreaElement | HTMLInputElement>) => {
        setLocal(event.target.value);
        informChange(name, event.target.value);
    };
    const inputProps = {id, 'data-testid': id, value: local ?? draft.saved[name], onChange};

    return (
        <Container>
            <Label htmlFor={id}>{label}</Label>
            {multiline ? <InstructionsTextArea {...inputProps}/> : <ModelInput {...inputProps}/>}
            <HelpText>{helpText}</HelpText>
        </Container>
    );
};

export const AutoReplyInstructionsField = ({informChange}: Props) => (
    <AmbientField
        informChange={informChange}
        name='instructions'
        multiline={true}
        label={<FormattedMessage defaultMessage='Ambient instructions'/>}
        helpText={<FormattedMessage defaultMessage='Instructions for when the agent should reply. Used by both ambient modes.'/>}
    />
);

// TODO(ambient-poc): replace this free-text input with a channel-authorized
// model discovery/validation endpoint. POC assumption: the value is an
// unvalidated model id string.
export const AutoReplyAnalysisModelField = ({informChange}: Props) => (
    <AmbientField
        informChange={informChange}
        name='analysis_model'
        label={<FormattedMessage defaultMessage='Analysis model'/>}
        helpText={<FormattedMessage defaultMessage='Model id used to decide whether to reply. Used by both ambient modes.'/>}
    />
);

const Container = styled.div`
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 16px;
`;

const Label = styled.label`
    font-weight: 600;
    font-size: 14px;
`;

const HelpText = styled.div`
    color: rgba(var(--center-channel-color-rgb), 0.72);
    font-size: 12px;
`;

const fieldChrome = `
    width: 100%;
    padding: 6px 12px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.16);
    border-radius: 4px;
    background: var(--center-channel-bg);
    color: var(--center-channel-color);
    font-size: 14px;
    line-height: 20px;
    box-sizing: border-box;
    outline: none;

    &:focus {
        border-color: var(--button-bg);
    }
`;

const InstructionsTextArea = styled.textarea`
    ${fieldChrome}
    resize: vertical;
    min-height: 120px;
`;

const ModelInput = styled.input.attrs({type: 'text'})`
    ${fieldChrome}
`;
