import React from 'react';
import styled from 'styled-components';

import { TrashCanOutlineIcon } from '@mattermost/compass-icons/components';

import { ExternalTool } from './external_tool';

type Props = {
    externalTool: ExternalTool
    onChange: (externalTool: ExternalTool) => void
    onDelete: (externalTool: ExternalTool) => void
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

const ExternalToolForm = ({ externalTool, onChange, onDelete }: Props) => {
    return (
        <FormContainer>
            <FormHeader>
                {externalTool.Provider}
                <CloseButton
                    aria-label='Delete'
                    onClick={() => onDelete(externalTool)}
                    title='Delete'
                >
                    <TrashIcon />
                </CloseButton>
            </FormHeader>
            <FormBody>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-select'
                    >
                        {'Tool Service'}
                    </label>
                    <div className='col-sm-8'>
                        <select
                            id='ai-service-select'
                            className='form-control'
                            onChange={(e) => onChange({ ...externalTool, Provider: e.target.value })}
                            value={externalTool.Provider}
                        >
                            <option value='zapier'>{'Zapier'}</option>
                            <option value='n8n'>{'N8N'}</option>
                            <option value='superface'>{'Superface'}</option>
                        </select>
                    </div>
                </div>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='openai-org-id'
                    >
                        {'API Key'}
                    </label>
                    <div className='col-sm-8'>
                        <input
                            id='openai-org-id'
                            className='form-control'
                            type='password'
                            placeholder='API Key'
                            value={externalTool.AuthToken}
                            onChange={(e) => onChange({ ...externalTool, AuthToken: e.target.value })}
                        />
                    </div>
                </div>
                <div className='form-group'>
                    <label
                        className='control-label col-sm-4'
                        htmlFor='ai-service-default-model'
                    >
                        {'API URL'}
                    </label>
                    <div className='col-sm-8'>
                        <input
                            id='ai-service-default-model'
                            className='form-control'
                            type='text'
                            placeholder='API URL'
                            value={externalTool.URL}
                            onChange={(e) => onChange({ ...externalTool, URL: e.target.value })}
                        />
                    </div>
                </div>

            </FormBody>
        </FormContainer>
    );
};
export default ExternalToolForm;
