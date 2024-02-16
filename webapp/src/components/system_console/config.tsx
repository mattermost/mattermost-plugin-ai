import React, {useCallback} from 'react';
import styled from 'styled-components';

import {PlusIcon} from '@mattermost/compass-icons/components';

import {TertiaryButton} from '../assets/buttons';

import {useIsMultiLLMLicensed} from '@/license';

import {ServiceData} from './service';
import ServiceForm from './service_form';
import EnterpriseChip from './enterprise_chip';

type Value = {
    services: ServiceData[],
    llmBackend: string,
    transcriptBackend: string,
    imageGeneratorBackend: string,
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
    value: Value
    disabled: boolean
    config: any
    currentState: any
    license: any
    setByEnv: boolean
    onChange: (id: string, value: any) => void
    setSaveNeeded: () => void
}

const PanelContainer = styled.div`
	display: flex;
	flex-direction: column;
	padding: 32px;
	gap: 32px;
	border: 1px solid #ccc;
	background: white;
	border-radius: 4px;
	box-shadow: 0px 2px 3px 0px rgba(0, 0, 0, 0.08);
`;

const PanelHeader = styled.div`
	display: flex;
	flex-direction: column;
	gap: 4px;
`;

const PanelTitle = styled.div`
	font-size: 16px;
	font-weight: 600;
`;

const PanelSubtitle = styled.div`
	color: rgba(63, 67, 80, 0.72);
	font-size: 14px;
	font-weight: 400;
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

const EnterpriseChipContainer = styled.div`
	display: flex;
	flex-direction: row;
	align-items: center;
	gap: 8px;
`;

const defaultConfig = {
    services: [],
    llmBackend: '',
    transcriptBackend: '',
    imageGeneratorBackend: '',
    enableLLMTrace: false,
    enableUserRestrictions: false,
    allowPrivateChannels: false,
    allowedTeamIds: '',
    onlyUsersOnTeam: '',
};

type PanelProps = {
    title: string
    subtitle: string
    children: React.ReactNode
}

const Panel = (props: PanelProps) => {
    return (
        <PanelContainer>
            <PanelHeader>
                <PanelTitle>{props.title}</PanelTitle>
                <PanelSubtitle>{props.subtitle}</PanelSubtitle>
            </PanelHeader>
            <div>
                {props.children}
            </div>
        </PanelContainer>
    );
};

const Config = (props: Props) => {
    const value = props.value || defaultConfig;
    const currentServices = value.services;
    const multiLLMLicensed = useIsMultiLLMLicensed();
    const licenceAddDisabled = !multiLLMLicensed && currentServices.length > 0;

    const addNewService = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        const newService = {
            id: Math.random().toString(36).substring(2, 22),
            name: 'AI Engine',
            serviceName: 'openai',
            defaultModel: '',
            url: '',
            apiKey: '',
            orgId: '',
            username: '',
            password: '',
            tokenLimit: 0,
        };

        let counter = 1;
        for (;;) {
            let isNew = true;
            for (const service of currentServices) {
                if (service.name === newService.name) {
                    isNew = false;
                }
            }
            if (isNew) {
                break;
            }
            newService.name = `AI Engine ${counter}`;
            counter++;
        }
        if (value.services.length === 0) {
            props.onChange(props.id, {...value, services: [...currentServices, newService], llmBackend: newService.name, transcriptBackend: newService.name, imageGeneratorBackend: newService.name});
        } else {
            props.onChange(props.id, {...value, services: [...currentServices, newService]});
        }
    }, [value, currentServices]);

    const deleteService = (deletedService: ServiceData) => {
        const updatedServiceIdx = currentServices.indexOf(deletedService);
        if (updatedServiceIdx === -1) {
            throw new Error('Service not found');
        }
        let newValue = value;
        if (currentServices.length > 1) {
            if (value.llmBackend === deletedService.name) {
                newValue = {...newValue, llmBackend: value.services[0]?.name || ''};
            }
            if (value.imageGeneratorBackend === deletedService.name) {
                newValue = {...newValue, imageGeneratorBackend: value.services[0]?.name || ''};
            }
            if (value.transcriptBackend === deletedService.name) {
                newValue = {...newValue, transcriptBackend: value.services[0]?.name || ''};
            }
        } else {
            newValue = {...newValue, llmBackend: '', transcriptBackend: '', imageGeneratorBackend: ''};
        }
        props.onChange(props.id, {...newValue, services: [...currentServices.slice(0, updatedServiceIdx), ...currentServices.slice(updatedServiceIdx + 1)]});
        props.setSaveNeeded();
    };

    const changeService = (changedService: ServiceData) => {
        const updatedServiceIdx = currentServices.findIndex((s) => changedService.id === s.id);
        if (updatedServiceIdx === -1) {
            throw new Error('Service not found');
        }
        let newValue = value;
        if (value.llmBackend === currentServices[updatedServiceIdx].name) {
            newValue = {...newValue, llmBackend: changedService.name};
        }
        if (value.imageGeneratorBackend === currentServices[updatedServiceIdx].name) {
            newValue = {...newValue, imageGeneratorBackend: changedService.name};
        }
        if (value.transcriptBackend === currentServices[updatedServiceIdx].name) {
            newValue = {...newValue, transcriptBackend: changedService.name};
        }
        props.onChange(props.id, {...newValue, services: [...currentServices.slice(0, updatedServiceIdx), changedService, ...currentServices.slice(updatedServiceIdx + 1)]});
        props.setSaveNeeded();
    };

    return (
        <ConfigContainer>
            <Panel
                title='AI Services'
                subtitle='Multiple AI services can be configured with the AI plugin.'
            >
                {currentServices.map((service) => (
                    <ServiceForm
                        key={service.id}
                        service={service}
                        onDelete={deleteService}
                        onChange={changeService}
                    />
                ))}
                <EnterpriseChipContainer>
                    <TertiaryButton
                        onClick={addNewService}
                        disabled={licenceAddDisabled}
                    >
                        <PlusAIServiceIcon/>
                        {'Add AI Service'}
                    </TertiaryButton>
                    {licenceAddDisabled && (
                        <EnterpriseChip subtext={'Multiple AI services is available on Enterprise plans'}/>
                    )}
                </EnterpriseChipContainer>
            </Panel>
            <Panel
                title='AI functions'
                subtitle='If you have more than one AI server you can choose which service to use for each function.'
            >
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-llm-backend'
                    >
                        {'AI Large Language Model service'}
                    </label>
                    <div className='col-sm-8'>
                        <select
                            id='ai-llm-backend'
                            className={currentServices.length === 0 ? 'form-control disabled' : 'form-control'}
                            onChange={(e) => {
                                props.onChange(props.id, {...value, llmBackend: e.target.value});
                                props.setSaveNeeded();
                            }}
                            value={value.llmBackend}
                            disabled={currentServices.length === 0}
                        >
                            {currentServices.map((service) => (
                                <option
                                    key={service.id}
                                    value={service.name}
                                >
                                    {service.name}
                                </option>
                            ))}
                        </select>
                        {currentServices.length === 0 && (
                            <div className='help-text'>
                                <span>{'You need at least one AI services use this setting.'}</span>
                            </div>
                        )}
                    </div>
                </div>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-transcript-backend'
                    >
                        {'AI Audio/Video transcript service'}
                    </label>
                    <div className='col-sm-8'>
                        <select
                            id='ai-transcript-backend'
                            className={currentServices.length === 0 ? 'form-control disabled' : 'form-control'}
                            onChange={(e) => {
                                props.onChange(props.id, {...value, transcriptBackend: e.target.value});
                                props.setSaveNeeded();
                            }}
                            value={value.transcriptBackend}
                            disabled={currentServices.length === 0}
                        >
                            {currentServices.map((service) => (
                                <option
                                    key={service.id}
                                    value={service.name}
                                >
                                    {service.name}
                                </option>
                            ))}
                        </select>
                        {currentServices.length === 0 && (
                            <div className='help-text'>
                                <span>{'You need at least one AI services use this setting.'}</span>
                            </div>
                        )}
                    </div>
                </div>
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
