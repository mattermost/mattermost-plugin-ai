// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage, useIntl} from 'react-intl';

import {Button} from '@mattermost/compass-ui/components/button';
import {ErrorMessage} from '@mattermost/compass-ui/components/error-message';
import {Modal} from '@mattermost/compass-ui/components/modal';
import {Radio} from '@mattermost/compass-ui/components/radio';
import {SectionNotice} from '@mattermost/compass-ui/components/section-notice';
import {TextInput} from '@mattermost/compass-ui/components/text-input';

import {importAgent, previewAgentImport} from '@/client';
import {AnimatedModalShell, MODAL_SHEET_CLASS} from '@/components/animated_modal_shell';
import {SelectField} from '@/components/system_console/item';
import {
    AgentImportMCPServerMapping,
    AgentImportMode,
    AgentImportPreview,
    AgentImportRequest,
    ServiceInfo,
    UserAgent,
    DefaultMaxToolTurns,
    codePointLength,
} from '@/types/agents';

import {agentUsernameError} from './agent_username';

// Select value meaning "drop the tools of this MCP server" (sent as targetOrigin '').
const REMOVE_TOOLS_VALUE = '__remove__';
const INSTRUCTIONS_PREVIEW_CHARS = 240;

export type ImportedAgentResult = {
    agent: UserAgent;
    mode: AgentImportMode;
}

type Props = {
    services: ServiceInfo[];

    /** Agents the current user can manage; targets for "update an existing agent". */
    manageableAgents: UserAgent[];

    canCreate: boolean;

    /** Set when creating is blocked even though the user normally can (e.g. plan quota reached). */
    createDisabledReason?: string;
    onClose: () => void;
    onImported: (result: ImportedAgentResult) => void;
}

function readFileAsText(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
        reader.onerror = () => reject(reader.error ?? new Error('read failed'));
        reader.readAsText(file);
    });
}

function errorText(e: unknown, fallback: string): string {
    const message = (e as {message?: unknown})?.message;
    return typeof message === 'string' && message.trim() ? message.trim() : fallback;
}

const ImportAgentModal = ({services, manageableAgents, canCreate, createDisabledReason, onClose, onImported}: Props) => {
    const intl = useIntl();
    const fileInputRef = useRef<HTMLInputElement>(null);

    const createAllowed = canCreate && !createDisabledReason;
    const updateAllowed = manageableAgents.length > 0;

    const [fileName, setFileName] = useState('');
    const [loadingPreview, setLoadingPreview] = useState(false);
    const [fileError, setFileError] = useState('');
    const [preview, setPreview] = useState<AgentImportPreview | null>(null);
    const [mode, setMode] = useState<AgentImportMode>(createAllowed ? 'create' : 'update');
    const [targetAgentId, setTargetAgentId] = useState('');
    const [username, setUsername] = useState('');
    const [displayName, setDisplayName] = useState('');
    const [serviceId, setServiceId] = useState(services[0]?.id ?? '');
    const [model, setModel] = useState('');

    // Keyed by source origin; '' = not chosen yet, REMOVE_TOOLS_VALUE = drop the tools.
    const [mappings, setMappings] = useState<Record<string, string>>({});
    const [submitting, setSubmitting] = useState(false);
    const [submitError, setSubmitError] = useState('');

    // Only the latest file selection may apply its preview.
    const previewRequestRef = useRef(0);

    const handleClose = useCallback(() => {
        if (!submitting) {
            onClose();
        }
    }, [onClose, submitting]);

    useEffect(() => {
        setServiceId((prev) => prev || (services[0]?.id ?? ''));
    }, [services]);

    const handleFileChange = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];

        // Let choosing the same file again (e.g. after fixing it) fire another change.
        e.target.value = '';
        if (!file) {
            return;
        }
        const requestId = ++previewRequestRef.current;
        const isStale = () => requestId !== previewRequestRef.current;
        setFileName(file.name);
        setPreview(null);
        setFileError('');
        setSubmitError('');

        let parsed: unknown;
        try {
            parsed = JSON.parse(await readFileAsText(file));
        } catch {
            if (isStale()) {
                return;
            }
            setFileError(intl.formatMessage({defaultMessage: 'This file is not valid JSON.'}));
            return;
        }
        if (isStale()) {
            return;
        }
        if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
            setFileError(intl.formatMessage({defaultMessage: 'This file is not an agent export document.'}));
            return;
        }

        setLoadingPreview(true);
        try {
            const result = await previewAgentImport(parsed as AgentImportPreview['document']);
            if (isStale()) {
                return;
            }
            const manageableExisting = result.existingAgent && result.existingAgent.canManage &&
                manageableAgents.some((a) => a.id === result.existingAgent?.id) ? result.existingAgent : null;

            let initialMode: AgentImportMode = 'create';
            if (manageableExisting) {
                initialMode = 'update';
            } else if (!createAllowed) {
                initialMode = 'update';
            }
            setMode(initialMode);

            // Only the agent the document names is a safe default; any other
            // target must be chosen explicitly so one click never overwrites
            // an unrelated agent.
            setTargetAgentId(manageableExisting?.id ?? '');
            setUsername(result.document.agent.name ?? '');
            setDisplayName(result.document.agent.displayName ?? '');
            setModel('');
            const initialMappings: Record<string, string> = {};
            for (const server of result.mcpServers ?? []) {
                initialMappings[server.sourceOrigin] = server.autoTargetOrigin || '';
            }
            setMappings(initialMappings);
            setPreview(result);
        } catch (err) {
            if (!isStale()) {
                setFileError(errorText(err, intl.formatMessage({defaultMessage: 'Failed to read this agent file.'})));
            }
        } finally {
            if (!isStale()) {
                setLoadingPreview(false);
            }
        }
    }, [createAllowed, intl, manageableAgents]);

    const mcpServers = useMemo(() => preview?.mcpServers ?? [], [preview]);
    const availableServers = useMemo(() => preview?.availableMCPServers ?? [], [preview]);
    const autoEnableAll = preview?.document.agent.autoEnableNewMCPTools ?? false;
    const mappingRequired = !autoEnableAll && mcpServers.length > 0;
    const unresolvedMappings = mappingRequired && mcpServers.some((s) => !mappings[s.sourceOrigin]);

    const usernameError = mode === 'create' ? agentUsernameError(intl, username) : '';
    const createFieldsValid = !usernameError && displayName.trim() !== '' && serviceId !== '';
    const updateFieldsValid = targetAgentId !== '';
    const canSubmit = Boolean(preview) && !submitting && !unresolvedMappings &&
        (mode === 'create' ? createFieldsValid : updateFieldsValid);

    const handleImport = async () => {
        if (!preview || !canSubmit) {
            return;
        }
        const mcpServerMappings: AgentImportMCPServerMapping[] = mappingRequired ? mcpServers.map((s) => {
            const chosen = mappings[s.sourceOrigin] ?? '';
            return {
                sourceOrigin: s.sourceOrigin,
                targetOrigin: chosen === REMOVE_TOOLS_VALUE ? '' : chosen,
            };
        }) : [];

        const request: AgentImportRequest = mode === 'create' ? {
            document: preview.document,
            mode,
            username: username.trim(),
            displayName: displayName.trim(),
            serviceID: serviceId,
            model: model.trim(),
            mcpServerMappings,
        } : {
            document: preview.document,
            mode,
            agentID: targetAgentId,
            mcpServerMappings,
        };

        setSubmitting(true);
        setSubmitError('');
        try {
            const agent = await importAgent(request);
            onImported({agent, mode});
        } catch (e) {
            setSubmitError(errorText(e, intl.formatMessage({defaultMessage: 'Failed to import agent. Please try again.'})));
            setSubmitting(false);
        }
    };

    const doc = preview?.document.agent;
    const instructionsLength = doc ? codePointLength(doc.customInstructions ?? '') : 0;
    const instructionsPreview = doc ? Array.from(doc.customInstructions ?? '').slice(0, INSTRUCTIONS_PREVIEW_CHARS).join('') : '';

    return (
        <AnimatedModalShell
            show={true}
            zIndex={2000}
        >
            <Sheet className={MODAL_SHEET_CLASS}>
                <ImportModal
                    size='medium'
                    title={<FormattedMessage defaultMessage='Import agent'/>}
                    onClose={handleClose}
                    closeLabel={intl.formatMessage({defaultMessage: 'Close'})}
                    closeOnEscape={!submitting}
                    footer={(
                        <>
                            <Button
                                emphasis='tertiary'
                                onClick={handleClose}
                                disabled={submitting}
                            >
                                <FormattedMessage defaultMessage='Cancel'/>
                            </Button>
                            <Button
                                emphasis='primary'
                                onClick={handleImport}
                                disabled={!canSubmit}
                                loading={submitting}
                            >
                                {submitting ? <FormattedMessage defaultMessage='Importing...'/> : <FormattedMessage defaultMessage='Import'/>}
                            </Button>
                        </>
                    )}
                >
                    <ModalBody>
                        <Description>
                            <FormattedMessage defaultMessage='Import the instructions, tool settings, and MCP tools from an agent export file. The AI service, model, and access settings are not part of the file.'/>
                        </Description>

                        <Field>
                            <FieldLabel htmlFor='import-agent-file'>
                                <FormattedMessage defaultMessage='Agent file'/>
                            </FieldLabel>
                            <FileRow>
                                <Button
                                    emphasis='tertiary'
                                    onClick={() => fileInputRef.current?.click()}
                                    disabled={submitting || loadingPreview}
                                >
                                    <FormattedMessage defaultMessage='Choose file'/>
                                </Button>
                                <FileName>{fileName || intl.formatMessage({defaultMessage: 'No file selected'})}</FileName>
                                <HiddenFileInput
                                    id='import-agent-file'
                                    ref={fileInputRef}
                                    type='file'
                                    accept='.json,application/json'
                                    data-testid='import-agent-file-input'
                                    onChange={handleFileChange}
                                />
                            </FileRow>
                            {loadingPreview && (
                                <HelpText>
                                    <FormattedMessage defaultMessage='Reading file...'/>
                                </HelpText>
                            )}
                            {fileError && (
                                <ErrorMessage
                                    message={fileError}
                                    role='alert'
                                />
                            )}
                        </Field>

                        {preview && doc && (
                            <>
                                <Summary data-testid='import-summary'>
                                    <SummaryTitle>
                                        {doc.displayName || doc.name}
                                        {doc.name && <SummaryHandle>{`@${doc.name}`}</SummaryHandle>}
                                    </SummaryTitle>
                                    <SummaryLine>
                                        {intl.formatMessage(
                                            {defaultMessage: 'Exported from version {version}'},
                                            {version: preview.document.agentVersion},
                                        )}
                                    </SummaryLine>
                                    <SummaryLine>
                                        {intl.formatMessage(
                                            {defaultMessage: 'Custom instructions: {count, plural, =0 {none} one {# character} other {# characters}}'},
                                            {count: instructionsLength},
                                        )}
                                    </SummaryLine>
                                    {instructionsPreview && (
                                        <InstructionsPreview>
                                            {instructionsPreview}
                                            {instructionsLength > INSTRUCTIONS_PREVIEW_CHARS ? '…' : ''}
                                        </InstructionsPreview>
                                    )}
                                    <SummaryLine>
                                        {doc.disableTools ? (
                                            <FormattedMessage defaultMessage='Tools: disabled'/>
                                        ) : (
                                            intl.formatMessage(
                                                {defaultMessage: 'Tools: enabled, up to {turns} tool turns'},
                                                {turns: doc.maxToolTurns > 0 ? doc.maxToolTurns : DefaultMaxToolTurns},
                                            )
                                        )}
                                    </SummaryLine>
                                    <SummaryLine>
                                        {doc.autoEnableNewMCPTools ? (
                                            <FormattedMessage defaultMessage='MCP tools: all MCP tools'/>
                                        ) : (
                                            intl.formatMessage(
                                                {defaultMessage: 'MCP tools: {count, plural, =0 {none} one {# tool} other {# tools}}'},
                                                {count: doc.mcpTools?.length ?? 0},
                                            )
                                        )}
                                    </SummaryLine>
                                </Summary>

                                <Field>
                                    <FieldLabel as='span'>
                                        <FormattedMessage defaultMessage='Import as'/>
                                    </FieldLabel>
                                    <RadioRow>
                                        <Radio
                                            name='import-agent-mode'
                                            checked={mode === 'create'}
                                            disabled={!createAllowed || submitting}
                                            onChange={() => setMode('create')}
                                        >
                                            <FormattedMessage defaultMessage='Create a new agent'/>
                                        </Radio>
                                        <Radio
                                            name='import-agent-mode'
                                            checked={mode === 'update'}
                                            disabled={!updateAllowed || submitting}
                                            onChange={() => setMode('update')}
                                        >
                                            <FormattedMessage defaultMessage='Update an existing agent'/>
                                        </Radio>
                                    </RadioRow>
                                    {createDisabledReason && canCreate && (
                                        <HelpText>{createDisabledReason}</HelpText>
                                    )}
                                </Field>

                                {mode === 'create' && (
                                    <>
                                        <Field>
                                            <FieldLabel htmlFor='import-agent-username'>
                                                <FormattedMessage defaultMessage='Username'/>
                                            </FieldLabel>
                                            <TextInput
                                                id='import-agent-username'
                                                value={username}
                                                disabled={submitting}
                                                onChange={(e) => setUsername(e.target.value)}
                                            />
                                            {usernameError && (
                                                <ErrorMessage
                                                    message={usernameError}
                                                    role='alert'
                                                />
                                            )}
                                        </Field>
                                        <Field>
                                            <FieldLabel htmlFor='import-agent-display-name'>
                                                <FormattedMessage defaultMessage='Display name'/>
                                            </FieldLabel>
                                            <TextInput
                                                id='import-agent-display-name'
                                                value={displayName}
                                                disabled={submitting}
                                                onChange={(e) => setDisplayName(e.target.value)}
                                            />
                                        </Field>
                                        <Field>
                                            <FieldLabel htmlFor='import-agent-service'>
                                                <FormattedMessage defaultMessage='AI service'/>
                                            </FieldLabel>
                                            <SelectField
                                                id='import-agent-service'
                                                value={serviceId}
                                                disabled={submitting}
                                                onChange={setServiceId}
                                                options={services.map((s) => ({value: s.id, label: s.name}))}
                                                placeholder={services.length === 0 ? intl.formatMessage({defaultMessage: 'No AI services available'}) : intl.formatMessage({defaultMessage: 'Select an AI service...'})}
                                            />
                                        </Field>
                                        <Field>
                                            <FieldLabel htmlFor='import-agent-model'>
                                                <FormattedMessage defaultMessage='Model (optional)'/>
                                            </FieldLabel>
                                            <TextInput
                                                id='import-agent-model'
                                                value={model}
                                                disabled={submitting}
                                                placeholder={intl.formatMessage({defaultMessage: 'Service default'})}
                                                onChange={(e) => setModel(e.target.value)}
                                            />
                                        </Field>
                                    </>
                                )}

                                {mode === 'update' && (
                                    <Field>
                                        <FieldLabel htmlFor='import-agent-target'>
                                            <FormattedMessage defaultMessage='Agent to update'/>
                                        </FieldLabel>
                                        <SelectField
                                            id='import-agent-target'
                                            value={targetAgentId}
                                            disabled={submitting}
                                            onChange={setTargetAgentId}
                                            options={manageableAgents.map((a) => ({value: a.id, label: `${a.displayName} (@${a.name})`}))}
                                            placeholder={intl.formatMessage({defaultMessage: 'Select an agent...'})}
                                        />
                                        <HelpText>
                                            <FormattedMessage defaultMessage='Only the instructions and tool settings are replaced. The name, AI service, model, and access settings stay as they are.'/>
                                        </HelpText>
                                    </Field>
                                )}

                                {mappingRequired && (
                                    <Field>
                                        <FieldLabel as='span'>
                                            <FormattedMessage defaultMessage='MCP servers'/>
                                        </FieldLabel>
                                        <HelpText>
                                            <FormattedMessage defaultMessage='Choose which MCP server on this instance provides each set of tools, or remove them.'/>
                                        </HelpText>
                                        <MappingTable data-testid='import-mcp-mappings'>
                                            {mcpServers.map((server, index) => {
                                                const selectId = `import-mcp-mapping-${index}`;
                                                const value = mappings[server.sourceOrigin] ?? '';
                                                const knownTarget = !value || value === REMOVE_TOOLS_VALUE ||
                                                availableServers.some((s) => s.origin === value);
                                                return (
                                                    <MappingRow key={server.sourceOrigin}>
                                                        <MappingSource>
                                                            <MappingName>
                                                                <label htmlFor={selectId}>{server.sourceName || server.sourceOrigin}</label>
                                                            </MappingName>
                                                            <MappingOrigin>{server.sourceOrigin}</MappingOrigin>
                                                            <MappingOrigin>
                                                                {intl.formatMessage(
                                                                    {defaultMessage: '{count, plural, one {# tool} other {# tools}}'},
                                                                    {count: server.toolNames.length},
                                                                )}
                                                            </MappingOrigin>
                                                        </MappingSource>
                                                        <SelectField
                                                            id={selectId}
                                                            value={value}
                                                            disabled={submitting}
                                                            onChange={(next) => setMappings((prev) => ({...prev, [server.sourceOrigin]: next}))}
                                                            options={[
                                                                ...availableServers.map((s) => ({value: s.origin, label: `${s.name} (${s.origin})`})),
                                                                ...(knownTarget ? [] : [{value, label: value}]),
                                                                {value: REMOVE_TOOLS_VALUE, label: intl.formatMessage({defaultMessage: 'Remove these tools'})},
                                                            ]}
                                                            placeholder={intl.formatMessage({defaultMessage: 'Select a server...'})}
                                                        />
                                                    </MappingRow>
                                                );
                                            })}
                                        </MappingTable>
                                    </Field>
                                )}
                            </>
                        )}

                        {submitError && (
                            <SectionNotice
                                type='danger'
                                title={submitError}
                                role='alert'
                            />
                        )}
                    </ModalBody>
                </ImportModal>
            </Sheet>
        </AnimatedModalShell>
    );
};

const Sheet = styled.div`
    display: flex;
    max-width: calc(100vw - 32px);
`;

const ImportModal = styled(Modal)`
    && {
        max-width: 100%;
        max-height: calc(100vh - 64px);
    }
`;

const ModalBody = styled.div`
    display: flex;
    flex-direction: column;
    gap: 20px;
`;

const Description = styled.p`
    margin: 0;
    font-size: 14px;
    line-height: 20px;
    color: rgba(var(--center-channel-color-rgb), 0.75);
`;

const Field = styled.div`
    display: flex;
    flex-direction: column;
    gap: 8px;
`;

const FieldLabel = styled.label`
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    line-height: 20px;
    color: var(--center-channel-color);
`;

const FileRow = styled.div`
    display: flex;
    align-items: center;
    gap: 12px;
`;

const FileName = styled.span`
    min-width: 0;
    font-size: 14px;
    color: rgba(var(--center-channel-color-rgb), 0.72);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
`;

const HiddenFileInput = styled.input`
    display: none;
`;

const HelpText = styled.div`
    font-size: 12px;
    line-height: 16px;
    color: rgba(var(--center-channel-color-rgb), 0.72);
`;

const Summary = styled.div`
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px 16px;
    border-radius: 4px;
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    background: rgba(var(--center-channel-color-rgb), 0.04);
`;

const SummaryTitle = styled.div`
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
    color: var(--center-channel-color);
`;

const SummaryHandle = styled.span`
    margin-left: 8px;
    font-size: 14px;
    font-weight: 400;
    color: rgba(var(--center-channel-color-rgb), 0.72);
`;

const SummaryLine = styled.div`
    font-size: 13px;
    line-height: 20px;
    color: var(--center-channel-color);
`;

const InstructionsPreview = styled.div`
    padding: 8px 12px;
    max-height: 96px;
    overflow: auto;
    white-space: pre-wrap;
    overflow-wrap: break-word;
    word-break: normal;
    font-size: 12px;
    line-height: 16px;
    border-radius: 4px;
    background: var(--center-channel-bg);
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
    color: rgba(var(--center-channel-color-rgb), 0.84);
`;

const RadioRow = styled.div`
    display: flex;
    flex-direction: column;
    gap: 8px;
`;

const MappingTable = styled.div`
    display: flex;
    flex-direction: column;
    gap: 12px;
`;

const MappingRow = styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    align-items: center;
`;

const MappingSource = styled.div`
    display: flex;
    flex-direction: column;
    min-width: 0;
`;

const MappingName = styled.div`
    font-size: 14px;
    font-weight: 600;
    line-height: 20px;

    label {
        margin: 0;
        font-weight: 600;
    }
`;

const MappingOrigin = styled.div`
    font-size: 12px;
    line-height: 16px;
    color: rgba(var(--center-channel-color-rgb), 0.72);
    overflow-wrap: break-word;
    word-break: normal;
`;

export default ImportAgentModal;
