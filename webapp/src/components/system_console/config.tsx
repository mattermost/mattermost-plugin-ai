import React, {useEffect, useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage, useIntl} from 'react-intl';

import {setUserProfilePictureByUsername} from '@/client';

import {ServiceData} from './service';
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
        <span>
            <FormattedMessage
                defaultMessage='To report a bug or to provide feedback, <link>create a new issue in the plugin repository</link>.'
                values={{link: (chunks: any) => (
                    <a
                        target={'_blank'}
                        rel={'noopener noreferrer'}
                        href='http://github.com/mattermost/mattermost-plugin-ai/issues'
                    >
                        {chunks}
                    </a>
                )}}
            />
        </span>
    </MessageContainer>
);

const Config = (props: Props) => {
    const value = props.value || defaultConfig;
    const [avatarUpdates, setAvatarUpdates] = useState<{[key: string]: File}>({});
    const intl = useIntl();

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
                title={intl.formatMessage({defaultMessage: 'AI Bots'})}
                subtitle={intl.formatMessage({defaultMessage: 'Multiple AI services can be configured below.'})}
            >
                <Bots
                    bots={props.value.bots ?? []}
                    onChange={(bots: LLMBotConfig[]) => {
                        if (value.bots.findIndex((bot) => bot.name === value.defaultBotName) === -1) {
                            props.onChange(props.id, {...value, bots, defaultBotName: bots[0].name});
                        } else {
                            props.onChange(props.id, {...value, bots});
                        }
                        props.setSaveNeeded();
                    }}
                    botChangedAvatar={botChangedAvatar}
                />
                <PanelFooterText>
                    <FormattedMessage defaultMessage='AI services are third-party services. Mattermost is not responsible for service output.'/>
                </PanelFooterText>
            </Panel>
            <Panel
                title={intl.formatMessage({defaultMessage: 'AI Functions'})}
                subtitle={intl.formatMessage({defaultMessage: 'Choose a default bot.'})}
            >
                <ItemList>
                    <SelectionItem
                        label={intl.formatMessage({defaultMessage: 'Default bot'})}
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
                title={intl.formatMessage({defaultMessage: 'User restrictions (experimental)'})}
                subtitle={intl.formatMessage({defaultMessage: 'Restrict where Copilot can be used.'})}
            >
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                    >
                        <FormattedMessage defaultMessage='Enable User Restrictions:'/>
                    </label>
                    <div className='col-sm-8'>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='true'
                                checked={value.enableUserRestrictions}
                                onChange={() => props.onChange(props.id, {...value, enableUserRestrictions: true})}
                            />
                            <span><FormattedMessage defaultMessage='true'/></span>
                        </label>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='false'
                                checked={!value.enableUserRestrictions}
                                onChange={() => props.onChange(props.id, {...value, enableUserRestrictions: false})}
                            />
                            <span><FormattedMessage defaultMessage='false'/></span>
                        </label>
                        <div className='help-text'>
                            <span>
                                <FormattedMessage defaultMessage='Global flag for all below settings.'/>
                            </span>
                        </div>
                    </div>
                </div>
                {value.enableUserRestrictions && (
                    <>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                            >
                                <FormattedMessage defaultMessage='Allow Private Channels:'/>
                            </label>
                            <div className='col-sm-8'>
                                <label className='radio-inline'>
                                    <input
                                        type='radio'
                                        value='true'
                                        checked={value.allowPrivateChannels}
                                        onChange={() => props.onChange(props.id, {...value, allowPrivateChannels: true})}
                                    />
                                    <span><FormattedMessage defaultMessage='true'/></span>
                                </label>
                                <label className='radio-inline'>
                                    <input
                                        type='radio'
                                        value='false'
                                        checked={!value.allowPrivateChannels}
                                        onChange={() => props.onChange(props.id, {...value, allowPrivateChannels: false})}
                                    />
                                    <span>
                                        <FormattedMessage defaultMessage='false'/>
                                    </span>
                                </label>
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='ai-allow-team-ids'
                            >
                                <FormattedMessage defaultMessage='Allow Team IDs (csv):'/>
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
                                <FormattedMessage defaultMessage='Only Users on Team:'/>
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
                title={intl.formatMessage({defaultMessage: 'Debug'})}
                subtitle=''
            >
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-name'
                    >
                        <FormattedMessage defaultMessage='Enable LLM Trace:'/>
                    </label>
                    <div className='col-sm-8'>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='true'
                                checked={value.enableLLMTrace}
                                onChange={() => props.onChange(props.id, {...value, enableLLMTrace: true})}
                            />
                            <span><FormattedMessage defaultMessage='true'/></span>
                        </label>
                        <label className='radio-inline'>
                            <input
                                type='radio'
                                value='false'
                                checked={!value.enableLLMTrace}
                                onChange={() => props.onChange(props.id, {...value, enableLLMTrace: false})}
                            />
                            <span><FormattedMessage defaultMessage='false'/></span>
                        </label>
                        <div className='help-text'>
                            <span>
                                <FormattedMessage defaultMessage='Enable tracing of LLM requests. Outputs full conversation data to the logs.'/>
                            </span>
                        </div>
                    </div>
                </div>
            </Panel>
        </ConfigContainer>
    );
};
export default Config;
