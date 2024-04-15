import React, {useCallback, useEffect, useState} from 'react';
import styled from 'styled-components';

import {PlusIcon} from '@mattermost/compass-icons/components';

import {TertiaryButton} from '../assets/buttons';

import {useIsMultiLLMLicensed} from '@/license';

import {Pill} from '../pill';

import {setUserProfilePictureByUsername} from '@/client';

import {ServiceData} from './service';
import ServiceForm from './service_form';
import EnterpriseChip from './enterprise_chip';
import Panel, {PanelFooterText} from './panel';
import Bots, {firstNewBot} from './bots';
import {LLMBotConfig} from './bot';
import {ItemList, SelectionItem, SelectionItemOption} from './item';
import NoBotsPage from './no_bots_page';

type Config = {
    services: ServiceData[],
    bots: LLMBotConfig[],
    defaultBotName: string,
    transcriptBackend: string,
    enableLLMTrace: boolean,
    enableCallSummary: boolean,

    enableUserRestrictions: boolean
    allowPrivateChannels: boolean
    allowedTeamIds: string
    onlyUsersOnTeam: string
}

type Props = {
    id: string
    label: string
    helpText: React.ReactNode
    value: Config
    disabled: boolean
    config: any
    currentState: any
    license: any
    setByEnv: boolean
    onChange: (id: string, value: any) => void
    setSaveNeeded: () => void
    registerSaveAction: (action: () => Promise<{error?: {message?: string}}>) => void
    unRegisterSaveAction: (action: () => Promise<{error?: {message?: string}}>) => void
}

const MessageContainer = styled.div`
	display: flex;
	align-items: center;
	flex-direction: row;
	gap: 5px;
	padding: 10px 12px;
	background: white;
	border-radius: 4px;
	border: 1px solid rgba(63, 67, 80, 0.08);
`;

const PlusAIServiceIcon = styled(PlusIcon)`
	width: 18px;
	height: 18px;
	margin-right: 8px;
`;

const ConfigContainer = styled.div`
	display: flex;
	flex-direction: column;
	gap: 20px;
`;

const defaultConfig = {
    services: [],
    llmBackend: '',
    transcriptBackend: '',
    enableLLMTrace: false,
    enableUserRestrictions: false,
    allowPrivateChannels: false,
    allowedTeamIds: '',
    onlyUsersOnTeam: '',
};

const BetaMessage = () => (
    <MessageContainer>
        <Pill>
            {'BETA'}
        </Pill>
        <span>
            {'This plugin is currently in beta. To report a bug or to provide feedback, '}
            <a
                target={'_blank'}
                rel={'noopener noreferrer'}
                href='http://github.com/mattermost/mattermost-plugin-ai/issues'
            >
                {'create a new issue in the plugin repository'}
            </a>
        </span>
    </MessageContainer>
);

const Config = (props: Props) => {
    const value = props.value || defaultConfig;
    const currentServices = value.services;
    const multiLLMLicensed = useIsMultiLLMLicensed();
    const licenceAddDisabled = !multiLLMLicensed && currentServices.length > 0;
    const [avatarUpdates, setAvatarUpdates] = useState<{[key: string]: File}>({});

    useEffect(() => {
        const save = async () => {
            Object.keys(avatarUpdates).map((username: string) => setUserProfilePictureByUsername(username, avatarUpdates[username]));
            return {};
        };
        props.registerSaveAction(save);
        return () => {
            props.unRegisterSaveAction(save);
        };
    }, [avatarUpdates]);

    const botChangedAvatar = (bot: LLMBotConfig, image: File) => {
        setAvatarUpdates((prev: {[key: string]: File}) => ({...prev, [bot.name]: image}));
        props.setSaveNeeded();
    };

    const addFirstBot = () => {
        const id = Math.random().toString(36).substring(2, 22);
        props.onChange(props.id, {
            ...value,
            bots: [{
                ...firstNewBot,
                id,
            }],
        });
    };

    if (!props.value?.bots || props.value.bots.length === 0) {
        return (
            <ConfigContainer>
                <BetaMessage/>
                <NoBotsPage onAddBotPressed={addFirstBot}/>
            </ConfigContainer>
        );
    }

    return (
        <ConfigContainer>
            <BetaMessage/>
            <Panel
                title='AI Bots'
                subtitle='Multiple AI services can be configured below.'
            >
                <Bots
                    bots={props.value.bots ?? []}
                    onChange={(bots: LLMBotConfig[]) => props.onChange(props.id, {...value, bots})}
                    botChangedAvatar={botChangedAvatar}
                />
                <PanelFooterText>
                    {'AI services are third party services; Mattermost is not responsible for output.'}
                </PanelFooterText>
            </Panel>
            <Panel
                title='AI functions'
                subtitle='Choose which bot you want to be the default for each function.'
            >
                <ItemList>
                    <SelectionItem
                        label='Default bot'
                        value={value.defaultBotName}
                        onChange={(e) => {
                            props.onChange(props.id, {...value, defaultBotName: e.target.value});
                            props.setSaveNeeded();
                        }}
                    >
                        {props.value.bots.map((bot: LLMBotConfig) => (
                            <SelectionItemOption
                                key={bot.name}
                                value={bot.name}
                            >
                                {bot.displayName}
                            </SelectionItemOption>
                        ))}
                    </SelectionItem>
                </ItemList>
            </Panel>

            <Panel
                title='User restrictions (experimental)'
                subtitle='Enable restrictions to allow or not users to use AI in this instance.'
            >
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                    >
                        {'Enable User Restrictions:'}
                    </label>
                    <div className='col-sm-8'>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='true'
                                checked={value.enableUserRestrictions}
                                onChange={() => props.onChange(props.id, {...value, enableUserRestrictions: true})}
                            />
                            <span>{'true'}</span>
                        </label>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='false'
                                checked={!value.enableUserRestrictions}
                                onChange={() => props.onChange(props.id, {...value, enableUserRestrictions: false})}
                            />
                            <span>{'false'}</span>
                        </label>
                        <div className='help-text'><span>{'Global flag for all below settings.'}</span></div>
                    </div>
                </div>
                {value.enableUserRestrictions && (
                    <>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                            >
                                {'Allow Private Channels:'}
                            </label>
                            <div className='col-sm-8'>
                                <label className='radio-inline'>
                                    <input
                                        type='radio'
                                        value='true'
                                        checked={value.allowPrivateChannels}
                                        onChange={() => props.onChange(props.id, {...value, allowPrivateChannels: true})}
                                    />
                                    <span>{'true'}</span>
                                </label>
                                <label className='radio-inline'>
                                    <input
                                        type='radio'
                                        value='false'
                                        checked={!value.allowPrivateChannels}
                                        onChange={() => props.onChange(props.id, {...value, allowPrivateChannels: false})}
                                    />
                                    <span>{'false'}</span>
                                </label>
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='ai-allow-team-ids'
                            >
                                {'Allow Team IDs (csv):'}
                            </label>
                            <div className='col-sm-8'>
                                <input
                                    id='ai-allow-team-ids'
                                    className='form-control'
                                    type='text'
                                    value={value.allowedTeamIds}
                                    onChange={(e) => props.onChange(props.id, {...value, allowedTeamIds: e.target.value})}
                                />
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='ai-only-users-on-team'
                            >
                                {'Only Users on Team:'}
                            </label>
                            <div className='col-sm-8'>
                                <input
                                    id='ai-only-users-on-team'
                                    className='form-control'
                                    type='text'
                                    value={value.onlyUsersOnTeam}
                                    onChange={(e) => props.onChange(props.id, {...value, onlyUsersOnTeam: e.target.value})}
                                />
                            </div>
                        </div>
                    </>
                )}
            </Panel>

            <Panel
                title='Debug'
                subtitle=''
            >
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-name'
                    >
                        {'Enable LLM Trace:'}
                    </label>
                    <div className='col-sm-8'>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='true'
                                checked={value.enableLLMTrace}
                                onChange={() => props.onChange(props.id, {...value, enableLLMTrace: true})}
                            />
                            <span>{'true'}</span>
                        </label>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='false'
                                checked={!value.enableLLMTrace}
                                onChange={() => props.onChange(props.id, {...value, enableLLMTrace: false})}
                            />
                            <span>{'false'}</span>
                        </label>
                        <div className='help-text'><span>{'Enable tracing of LLM requests. Outputs whole conversations to the logs.'}</span></div>
                    </div>
                </div>
            </Panel>
        </ConfigContainer>
    );
};
export default Config;
