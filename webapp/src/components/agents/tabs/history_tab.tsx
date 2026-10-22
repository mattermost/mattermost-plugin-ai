// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage, useIntl, type IntlShape} from 'react-intl';

import {UserProfile} from '@mattermost/types/users';

import {Button} from '@mattermost/compass-ui/components/button';
import {Checkbox} from '@mattermost/compass-ui/components/checkbox';
import {SectionNotice} from '@mattermost/compass-ui/components/section-notice';
import {Tag} from '@mattermost/compass-ui/components/tag';

import {deleteAgentVersions, getAgentVersion, getAgentVersions, getProfilesByIds, restoreAgentVersion} from '@/client';
import {AgentVersionDetail, AgentVersionList, ServiceInfo, UserAgent} from '@/types/agents';
import ConfirmationDialog from '@/components/confirmation_dialog';
import {UserAccessLevel} from '@/components/system_console/bot';

import {changedFieldLabels, versionSourceLabel} from '../agent_version_labels';
import VersionSnapshot from '../version_snapshot';

const RESTORE_DIALOG_TITLE_ID = 'restore-agent-version-title';
const DELETE_DIALOG_TITLE_ID = 'delete-agent-versions-title';

// The versions a confirmed delete removes: listed ones, or every previous one.
type PendingDelete = number[] | 'all';

type Props = {
    agentId: string;
    services: ServiceInfo[];

    /** True when the editor has unsaved changes that a restore would discard. */
    isDirty: boolean;

    /** The saved agent's user access level, which a restore replaces. */
    currentUserAccessLevel: UserAccessLevel;

    /** Called with the agent returned by the restore endpoint. */
    onRestored: (agent: UserAgent) => void;
}

function userDisplayName(user: UserProfile): string {
    const fullName = [user.first_name, user.last_name].filter(Boolean).join(' ').trim();
    return user.nickname || fullName || user.username;
}

function errorMessage(e: unknown, fallback: string): string {
    const message = (e as {message?: unknown})?.message;
    return typeof message === 'string' && message.trim() ? message.trim() : fallback;
}

function formatTimestamp(intl: IntlShape, timestamp: number): string {
    return intl.formatDate(timestamp, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
    });
}

function hasVersions(list: AgentVersionList): boolean {
    return list.versions.length > 0 && list.currentVersion > 0;
}

const HistoryTab = ({agentId, services, isDirty, currentUserAccessLevel, onRestored}: Props) => {
    const intl = useIntl();

    // Effects below must not re-run when a new intl object is created.
    const intlRef = useRef(intl);
    intlRef.current = intl;

    const [list, setList] = useState<AgentVersionList | null>(null);
    const [listError, setListError] = useState('');
    const [selected, setSelected] = useState<number | null>(null);
    const [detail, setDetail] = useState<AgentVersionDetail | null>(null);
    const [detailError, setDetailError] = useState('');
    const [authors, setAuthors] = useState<Record<string, string>>({});
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [restoring, setRestoring] = useState(false);
    const [restoreError, setRestoreError] = useState('');
    const [restoredNotice, setRestoredNotice] = useState('');
    const [loadingOlder, setLoadingOlder] = useState(false);
    const [olderError, setOlderError] = useState('');
    const [selecting, setSelecting] = useState(false);
    const [checkedVersions, setCheckedVersions] = useState<number[]>([]);
    const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null);
    const [deleting, setDeleting] = useState(false);
    const [deleteError, setDeleteError] = useState('');
    const [deletedNotice, setDeletedNotice] = useState('');
    const requestedAuthors = useRef(new Set<string>());
    const mountedRef = useRef(true);

    // Bumped by every first-page load so that page responses requested before
    // it (e.g. older pages of the list a restore replaced) are dropped.
    const listGenerationRef = useRef(0);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

    const loadList = useCallback(async (selectNewest: boolean) => {
        listGenerationRef.current += 1;
        const generation = listGenerationRef.current;
        setLoadingOlder(false);
        setOlderError('');
        try {
            setListError('');
            const result = await getAgentVersions(agentId);
            if (!mountedRef.current || generation !== listGenerationRef.current) {
                return;
            }
            const versions = result.versions ?? [];
            const next = {...result, versions};
            setList(next);
            setSelected((prev) => {
                if (!hasVersions(next)) {
                    return null;
                }
                if (selectNewest || prev === null || !versions.some((v) => v.version === prev)) {
                    return result.currentVersion;
                }
                return prev;
            });
        } catch (e) {
            if (mountedRef.current && generation === listGenerationRef.current) {
                setListError(errorMessage(e, intlRef.current.formatMessage({defaultMessage: 'Failed to load version history.'})));
            }
        }
    }, [agentId]);

    const loadOlder = async () => {
        if (!list || loadingOlder || list.versions.length === 0) {
            return;
        }
        const generation = listGenerationRef.current;
        const before = list.versions[list.versions.length - 1].version;
        setLoadingOlder(true);
        setOlderError('');
        try {
            const result = await getAgentVersions(agentId, {before});
            if (!mountedRef.current || generation !== listGenerationRef.current) {
                return;
            }
            setList((prev) => {
                // A reload since the request replaced the list, so this page
                // no longer continues it.
                if (!prev || prev.versions[prev.versions.length - 1]?.version !== before) {
                    return prev;
                }
                const older = (result.versions ?? []).filter((v) => v.version < before);
                return {...prev, versions: [...prev.versions, ...older], hasMore: Boolean(result.hasMore)};
            });
        } catch (e) {
            if (mountedRef.current && generation === listGenerationRef.current) {
                setOlderError(errorMessage(e, intl.formatMessage({defaultMessage: 'Failed to load older versions.'})));
            }
        } finally {
            if (mountedRef.current && generation === listGenerationRef.current) {
                setLoadingOlder(false);
            }
        }
    };

    useEffect(() => {
        loadList(true);
    }, [loadList]);

    useEffect(() => {
        if (!list) {
            return () => {
                // Nothing to resolve before the list loads
            };
        }
        const ids = list.versions.
            map((v) => v.createdBy).
            filter((id) => id && !requestedAuthors.current.has(id));
        const unique = Array.from(new Set(ids));
        if (unique.length === 0) {
            return () => {
                // All authors already requested
            };
        }
        unique.forEach((id) => requestedAuthors.current.add(id));
        getProfilesByIds(unique).then((profiles) => {
            if (!mountedRef.current) {
                return;
            }
            setAuthors((prev) => {
                const next = {...prev};
                for (const profile of profiles) {
                    next[profile.id] = userDisplayName(profile);
                }
                return next;
            });
        }).catch(() => {
            // Unresolved authors fall back to a generic label.
            unique.forEach((id) => requestedAuthors.current.delete(id));
        });
        return () => {
            // Requests are tracked in requestedAuthors; results apply while mounted.
        };
    }, [list]);

    useEffect(() => {
        if (selected === null) {
            return () => {
                // No version selected yet
            };
        }
        let cancelled = false;
        setDetail(null);
        setDetailError('');
        getAgentVersion(agentId, selected).then((result) => {
            if (!cancelled) {
                setDetail(result);
            }
        }).catch((e) => {
            if (!cancelled) {
                setDetailError(errorMessage(e, intlRef.current.formatMessage({defaultMessage: 'Failed to load this version.'})));
            }
        });
        return () => {
            cancelled = true;
        };
    }, [agentId, selected]);

    const authorLabel = (createdBy: string): string => {
        if (!createdBy) {
            return intl.formatMessage({defaultMessage: 'System'});
        }
        return authors[createdBy] ?? intl.formatMessage({defaultMessage: 'Unknown user'});
    };

    const handleSelect = (version: number) => {
        setSelected(version);
        setRestoreError('');
        setRestoredNotice('');
        setDeleteError('');
        setDeletedNotice('');
    };

    const toggleChecked = (version: number) => {
        setCheckedVersions((prev) => (prev.includes(version) ? prev.filter((v) => v !== version) : [...prev, version]));
    };

    const handleDeleteConfirm = async () => {
        if (pendingDelete === null || deleting) {
            return;
        }
        setDeleting(true);
        setDeleteError('');
        try {
            const deleted = await deleteAgentVersions(agentId, pendingDelete);
            setPendingDelete(null);
            setSelecting(false);
            setCheckedVersions([]);
            setRestoredNotice('');
            setDeletedNotice(intl.formatMessage(
                {defaultMessage: 'Deleted {count, plural, one {# version} other {# versions}}.'},
                {count: deleted},
            ));
            await loadList(false);
        } catch (e) {
            setPendingDelete(null);
            setDeleteError(errorMessage(e, intl.formatMessage({defaultMessage: 'Failed to delete versions. Please try again.'})));
        } finally {
            setDeleting(false);
        }
    };

    const handleRestoreConfirm = async () => {
        if (selected === null || restoring) {
            return;
        }
        const restoredVersion = selected;
        setRestoring(true);
        setRestoreError('');
        try {
            const agent = await restoreAgentVersion(agentId, restoredVersion);
            setConfirmOpen(false);
            setRestoredNotice(intl.formatMessage(
                {defaultMessage: 'Version {version} was restored as the current version.'},
                {version: restoredVersion},
            ));
            setDeletedNotice('');
            onRestored(agent);
            await loadList(true);
        } catch (e) {
            setConfirmOpen(false);
            setRestoreError(errorMessage(e, intl.formatMessage({defaultMessage: 'Failed to restore this version. Please try again.'})));
        } finally {
            setRestoring(false);
        }
    };

    if (listError && !list) {
        return (
            <SectionNotice
                type='danger'
                title={listError}
                role='alert'
            />
        );
    }

    if (!list) {
        return (
            <Muted>
                <FormattedMessage defaultMessage='Loading version history...'/>
            </Muted>
        );
    }

    if (!hasVersions(list)) {
        return (
            <Muted data-testid='version-history-empty'>
                <FormattedMessage defaultMessage='No versions recorded yet. Saving the agent creates the first version.'/>
            </Muted>
        );
    }

    const selectedSummary = list.versions.find((v) => v.version === selected);
    const isCurrentSelected = selected !== null && selected === list.currentVersion;
    const restoresRemovedPolicy = detail?.config?.userAccessLevel === UserAccessLevel.AttributeBased &&
        currentUserAccessLevel !== UserAccessLevel.AttributeBased;
    const hasPreviousVersions = list.hasMore || list.versions.some((v) => v.version !== list.currentVersion);
    const deleteCount = pendingDelete === 'all' ? 0 : pendingDelete?.length ?? 0;
    const deleteVersions = pendingDelete === 'all' ? '' : intl.formatList((pendingDelete ?? []).map(String), {type: 'conjunction'});
    const stopSelecting = () => {
        setSelecting(false);
        setCheckedVersions([]);
    };

    return (
        <Layout>
            <ListColumn>
                <ListToolbar>
                    {selecting ? (
                        <>
                            <SelectionCount role='status'>
                                {intl.formatMessage({defaultMessage: '{count} selected'}, {count: checkedVersions.length})}
                            </SelectionCount>
                            <ToolbarButtons>
                                <Button
                                    emphasis='tertiary'
                                    size='small'
                                    onClick={stopSelecting}
                                >
                                    <FormattedMessage defaultMessage='Cancel'/>
                                </Button>
                                <Button
                                    emphasis='primary'
                                    size='small'
                                    destructive={true}
                                    disabled={checkedVersions.length === 0 || deleting}
                                    onClick={() => setPendingDelete([...checkedVersions].sort((a, b) => a - b))}
                                >
                                    <FormattedMessage defaultMessage='Delete selected'/>
                                </Button>
                            </ToolbarButtons>
                        </>
                    ) : (
                        <>
                            <Button
                                emphasis='tertiary'
                                size='small'
                                disabled={!hasPreviousVersions}
                                onClick={() => setSelecting(true)}
                            >
                                <FormattedMessage defaultMessage='Select'/>
                            </Button>
                            <Button
                                emphasis='tertiary'
                                size='small'
                                destructive={true}
                                disabled={!hasPreviousVersions || deleting}
                                onClick={() => setPendingDelete('all')}
                            >
                                <FormattedMessage defaultMessage='Delete all previous'/>
                            </Button>
                        </>
                    )}
                </ListToolbar>
                <VersionList
                    aria-label={intl.formatMessage({defaultMessage: 'Version history'})}
                    data-testid='version-list'
                >
                    {list.versions.map((v) => {
                        const changed = changedFieldLabels(intl, v.changedFields);
                        const isCurrent = v.version === list.currentVersion;
                        return (
                            <VersionCard
                                key={v.version}
                                $selected={v.version === selected}
                            >
                                {selecting && (
                                    <CheckboxSlot>
                                        {!isCurrent && (
                                            <Checkbox
                                                checked={checkedVersions.includes(v.version)}
                                                onChange={() => toggleChecked(v.version)}
                                                aria-label={intl.formatMessage({defaultMessage: 'Select version {version}'}, {version: v.version})}
                                            />
                                        )}
                                    </CheckboxSlot>
                                )}
                                <VersionItem
                                    type='button'
                                    {...(v.version === selected ? {'aria-current': 'true' as const} : {})}
                                    onClick={() => handleSelect(v.version)}
                                >
                                    <VersionTitleRow>
                                        <VersionTitle>
                                            {intl.formatMessage({defaultMessage: 'Version {version}'}, {version: v.version})}
                                        </VersionTitle>
                                        {isCurrent && <CurrentTag/>}
                                    </VersionTitleRow>
                                    <VersionMeta>{versionSourceLabel(intl, v)}</VersionMeta>
                                    <VersionMeta>
                                        {intl.formatMessage(
                                            {defaultMessage: '{author} · {time}'},
                                            {author: authorLabel(v.createdBy), time: formatTimestamp(intl, v.createAt)},
                                        )}
                                    </VersionMeta>
                                    {changed.length > 0 && (
                                        <VersionMeta>
                                            {intl.formatMessage(
                                                {defaultMessage: 'Changed: {fields}'},
                                                {fields: changed.join(', ')},
                                            )}
                                        </VersionMeta>
                                    )}
                                </VersionItem>
                            </VersionCard>
                        );
                    })}
                </VersionList>
                {list.hasMore && (
                    <Button
                        emphasis='tertiary'
                        loading={loadingOlder}
                        onClick={loadOlder}
                    >
                        <FormattedMessage defaultMessage='Load older versions'/>
                    </Button>
                )}
                {olderError && (
                    <SectionNotice
                        type='danger'
                        title={olderError}
                        role='alert'
                    />
                )}
            </ListColumn>

            <DetailPane>
                {[restoredNotice, deletedNotice].filter(Boolean).map((message) => (
                    <SectionNotice
                        key={message}
                        type='success'
                        title={message}
                        role='status'
                    />
                ))}
                {[restoreError, deleteError, listError, detailError].filter(Boolean).map((message) => (
                    <SectionNotice
                        key={message}
                        type='danger'
                        title={message}
                        role='alert'
                    />
                ))}

                {selectedSummary && (
                    <DetailHeader>
                        <div>
                            <DetailTitle>
                                {intl.formatMessage({defaultMessage: 'Version {version}'}, {version: selectedSummary.version})}
                                {isCurrentSelected && <CurrentTag/>}
                            </DetailTitle>
                            <VersionMeta>
                                {intl.formatMessage(
                                    {defaultMessage: '{source} · {author} · {time}'},
                                    {
                                        source: versionSourceLabel(intl, selectedSummary),
                                        author: authorLabel(selectedSummary.createdBy),
                                        time: formatTimestamp(intl, selectedSummary.createAt),
                                    },
                                )}
                            </VersionMeta>
                        </div>
                        {!isCurrentSelected && (
                            <DetailActions>
                                <Button
                                    emphasis='tertiary'
                                    destructive={true}
                                    disabled={deleting || restoring}
                                    onClick={() => setPendingDelete([selectedSummary.version])}
                                >
                                    <FormattedMessage defaultMessage='Delete'/>
                                </Button>
                                <Button
                                    emphasis='primary'
                                    disabled={!detail || restoring || deleting}
                                    onClick={() => setConfirmOpen(true)}
                                >
                                    <FormattedMessage defaultMessage='Restore this version'/>
                                </Button>
                            </DetailActions>
                        )}
                    </DetailHeader>
                )}

                {!detail && !detailError && (
                    <Muted>
                        <FormattedMessage defaultMessage='Loading version...'/>
                    </Muted>
                )}
                {detail && (
                    <VersionSnapshot
                        config={detail.config ?? {}}
                        services={services}
                    />
                )}
            </DetailPane>

            <ConfirmationDialog
                show={confirmOpen}
                titleId={RESTORE_DIALOG_TITLE_ID}
                title={<FormattedMessage defaultMessage='Restore this version?'/>}
                message={(
                    <>
                        <p>
                            {intl.formatMessage(
                                {defaultMessage: 'Version {version} will be saved as a new version and the agent will start using it right away. The existing history is kept.'},
                                {version: selected ?? 0},
                            )}
                        </p>
                        {restoresRemovedPolicy && (
                            <WarningText role='alert'>
                                <FormattedMessage defaultMessage='This version uses attribute-based access. Its access policy was removed when access was changed, so nobody will be able to use the agent until you create a new policy on the Access tab.'/>
                            </WarningText>
                        )}
                        {isDirty && (
                            <WarningText role='alert'>
                                <FormattedMessage defaultMessage='You have unsaved changes in the editor. Restoring will discard them.'/>
                            </WarningText>
                        )}
                    </>
                )}
                confirmButtonText={restoring ? <FormattedMessage defaultMessage='Restoring...'/> : <FormattedMessage defaultMessage='Restore'/>}
                onConfirm={handleRestoreConfirm}
                onCancel={() => setConfirmOpen(false)}
                confirmPending={restoring}
                managedAccessibility={true}
                zIndex={2100}
            />
            <ConfirmationDialog
                show={pendingDelete !== null}
                titleId={DELETE_DIALOG_TITLE_ID}
                title={pendingDelete === 'all' ? (
                    <FormattedMessage defaultMessage='Delete all previous versions?'/>
                ) : intl.formatMessage(
                    {defaultMessage: '{count, plural, one {Delete version {versions}?} other {Delete # versions?}}'},
                    {count: deleteCount, versions: deleteVersions},
                )}
                message={pendingDelete === 'all' ? (
                    <FormattedMessage defaultMessage='Every version except the current one is permanently deleted. The agent keeps its current configuration. This cannot be undone.'/>
                ) : intl.formatMessage(
                    {defaultMessage: '{count, plural, one {Version {versions} is} other {Versions {versions} are}} permanently deleted from the history. The agent keeps its current configuration. This cannot be undone.'},
                    {count: deleteCount, versions: deleteVersions},
                )}
                confirmButtonText={deleting ? <FormattedMessage defaultMessage='Deleting...'/> : <FormattedMessage defaultMessage='Delete'/>}
                onConfirm={handleDeleteConfirm}
                onCancel={() => setPendingDelete(null)}
                confirmPending={deleting}
                isDestructive={true}
                managedAccessibility={true}
                zIndex={2100}
            />
        </Layout>
    );
};

const Layout = styled.div`
    display: flex;
    gap: 24px;
    align-items: flex-start;
`;

const ListColumn = styled.div`
    flex: 0 0 300px;
    display: flex;
    flex-direction: column;
    gap: 12px;
`;

const VersionList = styled.ol`
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
`;

const ListToolbar = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 32px;
`;

const ToolbarButtons = styled.div`
    display: flex;
    gap: 8px;
`;

const SelectionCount = styled.span`
    font-size: 12px;
    font-weight: 600;
    color: rgba(var(--center-channel-color-rgb), 0.72);
`;

const VersionCard = styled.li<{$selected: boolean}>`
    display: flex;
    align-items: flex-start;
    border-radius: 4px;
    overflow: hidden;
    border: 1px solid ${(p) => (p.$selected ? 'var(--button-bg)' : 'rgba(var(--center-channel-color-rgb), 0.12)')};
    background: ${(p) => (p.$selected ? 'rgba(var(--button-bg-rgb, 28, 88, 217), 0.08)' : 'var(--center-channel-bg)')};

    &:hover {
        background: ${(p) => (p.$selected ? 'rgba(var(--button-bg-rgb, 28, 88, 217), 0.08)' : 'rgba(var(--center-channel-color-rgb), 0.04)')};
    }
`;

// Fixed width so cards line up whether or not they have a checkbox.
const CheckboxSlot = styled.div`
    flex: 0 0 28px;
    display: flex;
    justify-content: flex-end;
    padding-top: 12px;
`;

const DetailActions = styled.div`
    display: flex;
    gap: 8px;
    flex-shrink: 0;
`;

const VersionItem = styled.button`
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    min-width: 0;
    padding: 10px 12px;
    text-align: left;
    cursor: pointer;
    color: var(--center-channel-color);
    border: none;
    background: transparent;
`;

const VersionTitleRow = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
`;

const VersionTitle = styled.span`
    font-size: 14px;
    font-weight: 600;
    line-height: 20px;
`;

const VersionMeta = styled.div`
    font-size: 12px;
    line-height: 16px;
    color: rgba(var(--center-channel-color-rgb), 0.72);
`;

const CurrentTag = () => (
    <Tag
        type='info'
        label={<FormattedMessage defaultMessage='Current'/>}
    />
);

const DetailPane = styled.div`
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
`;

const DetailHeader = styled.div`
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
`;

const DetailTitle = styled.h2`
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 4px;
    font-size: 18px;
    font-weight: 600;
    line-height: 24px;
    color: var(--center-channel-color);
`;

const Muted = styled.div`
    font-size: 14px;
    color: rgba(var(--center-channel-color-rgb), 0.64);
`;

const WarningText = styled.p`
    margin: 0;
    font-weight: 600;
    color: var(--dnd-indicator, #D24B4E);

    & + & {
        margin-top: 8px;
    }
`;

export default HistoryTab;
