import React from 'react';
import styled from 'styled-components';

import {TrashCanOutlineIcon} from '@mattermost/compass-icons/components';

import {ServiceData} from './service';

type Props = {
    service: ServiceData
    onChange: (service: ServiceData) => void
    onDelete: (service: ServiceData) => void
}

const FormContainer = styled.div`
	position: relative;
	border: 1px solid #ccc;
	margin-bottom: 20px;
	background: white;
	border-radius: 4px;
	box-shadow: 0px 2px 3px 0px rgba(0, 0, 0, 0.08);
`;

const CloseButton = styled.button`
	appearance: none;
	border: 0;
	padding: 0;
	cursor: pointer;
	background: transparent;
	width: 16px;
	height: 16px;
`;

const FormHeader = styled.div`
	display: flex;
	flex-direction: row;
	justify-content: space-between;
	align-items: flex-start;
	font-size: 14px;
	font-weight: 600;
	line-height: 20px;
	padding: 16px 20px;
	border-bottom: 1px solid rgba(63, 67, 80, 0.12);
`;

const TrashIcon = styled(TrashCanOutlineIcon)`
	width: 16px;
	height: 16px;
	color: #D24B4E;
`;

const FormBody = styled.div`
	padding: 20px;
`;

const ServiceForm = ({service, onChange, onDelete}: Props) => {
    return (
        <FormContainer>
            <FormHeader>
                {service.name}
                <CloseButton
                    aria-label='Delete'
                    onClick={() => onDelete(service)}
                    title='Delete'
                >
                    <TrashIcon/>
                </CloseButton>
            </FormHeader>
            <FormBody>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-name'
                    >
                        {'Name'}
                    </label>
                    <div className='col-sm-8'>
                        <input
                            id='ai-service-name'
                            className='form-control'
                            type='name'
                            placeholder='Name'
                            value={service.name}
                            onChange={(e) => onChange({...service, name: e.target.value})}
                        />
                    </div>
                </div>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-select'
                    >
                        {'AI Service'}
                    </label>
                    <div className='col-sm-8'>
                        <select
                            id='ai-service-select'
                            className='form-control'
                            onChange={(e) => onChange({...service, serviceName: e.target.value})}
                            value={service.serviceName}
                        >
                            <option value='openai'>{'OpenAI'}</option>
                            <option value='openaicompatible'>{'OpenAI Compatible'}</option>
                            <option value='anthropic'>{'Anthropic'}</option>
                            <option value='asksage'>{'Ask Sage'}</option>
                        </select>
                    </div>
                </div>
                {service.serviceName === 'openaicompatible' && (
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='ai-service-url'
                        >
                            {'API URL'}
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='ai-service-url'
                                className='form-control'
                                type='url'
                                placeholder='URL'
                                value={service.url}
                                onChange={(e) => onChange({...service, url: e.target.value})}
                            />
                        </div>
                    </div>
                )}
                {service.serviceName !== 'asksage' && (
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='ai-service-api-key'
                        >
                            {'API Key'}
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='ai-service-api-key'
                                className='form-control'
                                type='text'
                                placeholder='API Key'
                                value={service.apiKey}
                                onChange={(e) => onChange({...service, apiKey: e.target.value})}
                            />
                        </div>
                    </div>
                )}
                {service.serviceName === 'openai' && (
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='openai-org-id'
                        >
                            {'Organization ID'}
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='openai-org-id'
                                className='form-control'
                                type='text'
                                placeholder='Organization ID'
                                value={service.orgId}
                                onChange={(e) => onChange({...service, orgId: e.target.value})}
                            />
                        </div>
                    </div>
                )}
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-default-model'
                    >
                        {'Default Model'}
                    </label>
                    <div className='col-sm-8'>
                        <input
                            id='ai-service-default-model'
                            className='form-control'
                            type='text'
                            placeholder='Default Model'
                            value={service.defaultModel}
                            onChange={(e) => onChange({...service, defaultModel: e.target.value})}
                        />
                    </div>
                </div>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-default-model'
                    >
                        {'Token Limit'}
                    </label>
                    <div className='col-sm-8'>
                        <input
                            id='ai-service-default-model'
                            className='form-control'
                            type='number'
                            placeholder='Token Limit'
                            value={service.tokenLimit}
                            onChange={(e) => onChange({...service, tokenLimit: Number(e.target.value)})}
                        />
                    </div>
                </div>
                {service.serviceName === 'asksage' && (
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='ai-service-username'
                        >
                            {'Username'}
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='ai-service-username'
                                className='form-control'
                                type='text'
                                placeholder='Username'
                                value={service.username}
                                onChange={(e) => onChange({...service, username: e.target.value})}
                            />
                        </div>
                    </div>
                )}
                {service.serviceName === 'asksage' && (
                    <div className='form-group'>
                        <label
                            className='control-label col-sm-4'
                            htmlFor='ai-service-password'
                        >
                            {'Password'}
                        </label>
                        <div className='col-sm-8'>
                            <input
                                id='ai-service-password'
                                className='form-control'
                                type='password'
                                placeholder='Password'
                                value={service.password}
                                onChange={(e) => onChange({...service, password: e.target.value})}
                            />
                        </div>
                    </div>
                )}
            </FormBody>
        </FormContainer>
    );
};
export default ServiceForm;
