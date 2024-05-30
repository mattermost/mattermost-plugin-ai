import React, {useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage, useIntl} from 'react-intl';

import {TrashCanOutlineIcon, ChevronDownIcon, AlertOutlineIcon, ChevronUpIcon} from '@mattermost/compass-icons/components';

import IconAI from '../assets/icon_ai';
import {DangerPill} from '../pill';

import {ButtonIcon} from '../assets/buttons';

import {BooleanItem, ItemList, SelectionItem, SelectionItemOption, TextItem} from './item';
import AvatarItem from './avatar';

export type LLMService = {
    type: string
    apiURL: string
    apiKey: string
    orgId: string
    defaultModel: string
    username: string
    password: string
    tokenLimit: number
    streamingTimeoutSeconds: number
}

export type LLMBotConfig = {
    id: string
    name: string
    displayName: string
    service: LLMService
    customInstructions: string
    enableVision: boolean
}

type Props = {
    bot: LLMBotConfig
    onChange: (bot: LLMBotConfig) => void
    onDelete: () => void
    changedAvatar: (image: File) => void
}

const mapServiceTypeToDisplayName = new Map<string, string>([
    ['openai', 'OpenAI'],
    ['openaicompatible', 'OpenAI Compatible'],
    ['anthropic', 'Anthropic'],
    ['asksage', 'Ask Sage'],
]);

function serviceTypeToDisplayName(serviceType: string): string {
    return mapServiceTypeToDisplayName.get(serviceType) || serviceType;
}

const Bot = (props: Props) => {
    const [open, setOpen] = useState(false);
    const intl = useIntl();
    const missingInfo = props.bot.name === '' ||
		props.bot.displayName === '' ||
		props.bot.service.type === '' ||
		(props.bot.service.type !== 'asksage' && props.bot.service.apiKey === '') ||
		(props.bot.service.type === 'openaicompatible' && props.bot.service.apiURL === '');
    return (
        <BotContainer>
            <HeaderContainer onClick={() => setOpen((o) => !o)}>
                <IconAI/>
                <Title>
                    <NameText>
                        {props.bot.displayName}
                    </NameText>
                    <VerticalDivider/>
                    <ServiceTypeText>
                        {serviceTypeToDisplayName(props.bot.service.type)}
                    </ServiceTypeText>
                </Title>
                <Spacer/>
                {missingInfo && (
                    <DangerPill>
                        <AlertOutlineIcon/>
                        <FormattedMessage
                            id='bot.missingInfo'
                            defaultMessage='Missing information'
                        />
                    </DangerPill>
                )}
                <ButtonIcon
                    onClick={props.onDelete}
                >
                    <TrashIcon/>
                </ButtonIcon>
                {open ? <ChevronUpIcon/> : <ChevronDownIcon/>}
            </HeaderContainer>
            {open && (
                <ItemListContainer>
                    <ItemList>
                        <TextItem
                            label={intl.formatMessage({id: 'bot.displayName', defaultMessage: 'Display name'})}
                            value={props.bot.displayName}
                            onChange={(e) => props.onChange({...props.bot, displayName: e.target.value})}
                        />
                        <TextItem
                            label={intl.formatMessage({id: 'bot.botUsername', defaultMessage: 'Bot Username'})}
                            helptext={intl.formatMessage({id: 'bot.botUsernameHelpText', defaultMessage: 'Team members can mention this bot with this username'})}
                            value={props.bot.name}
                            onChange={(e) => props.onChange({...props.bot, name: e.target.value})}
                        />
                        <AvatarItem
                            botusername={props.bot.name}
                            changedAvatar={props.changedAvatar}
                        />
                        <SelectionItem
                            label={intl.formatMessage({id: 'bot.service', defaultMessage: 'Service'})}
                            value={props.bot.service.type}
                            onChange={(e) => props.onChange({...props.bot, service: {...props.bot.service, type: e.target.value}})}
                        >
                            <SelectionItemOption value='openai'>{'OpenAI'}</SelectionItemOption>
                            <SelectionItemOption value='openaicompatible'>{'OpenAI Compatible'}</SelectionItemOption>
                            <SelectionItemOption value='anthropic'>{'Anthropic'}</SelectionItemOption>
                            <SelectionItemOption value='asksage'>{'Ask Sage'}</SelectionItemOption>
                        </SelectionItem>
                        <ServiceItem
                            service={props.bot.service}
                            onChange={(service) => props.onChange({...props.bot, service})}
                        />
                        <TextItem
                            label={intl.formatMessage({id: 'bot.customInstructions', defaultMessage: 'Custom instructions'})}
                            placeholder={intl.formatMessage({id: 'bot.customInstructionsPlaceholder', defaultMessage: 'How would you like the AI to respond?'})}
                            multiline={true}
                            value={props.bot.customInstructions}
                            onChange={(e) => props.onChange({...props.bot, customInstructions: e.target.value})}
                        />
                        { (props.bot.service.type === 'openai' || props.bot.service.type === 'openaicompatible') && (
                            <BooleanItem
                                label={intl.formatMessage({id: 'bot.enableVision', defaultMessage: 'Enable Vision'})}
                                value={props.bot.enableVision}
                                onChange={(to: boolean) => props.onChange({...props.bot, enableVision: to})}
                                helpText={intl.formatMessage({id: 'bot.enableVisionHelpText', defaultMessage: 'Enable Vision to allow the bot to process images. Requires a compatible model.'})}
                            />
                        )}
                    </ItemList>
                </ItemListContainer>
            )}
        </BotContainer>
    );
};

type ServiceItemProps = {
    service: LLMService
    onChange: (service: LLMService) => void
}

const ServiceItem = (props: ServiceItemProps) => {
    const type = props.service.type;
    const intl = useIntl();
    const hasAPIKey = type !== 'asksage';
    const isOpenAIType = type === 'openai' || type === 'openaicompatible';
    return (
        <>
            {type === 'openaicompatible' && (
                <TextItem
                    label={intl.formatMessage({id: 'bot.apiURL', defaultMessage: 'API URL'})}
                    value={props.service.apiURL}
                    onChange={(e) => props.onChange({...props.service, apiURL: e.target.value})}
                />
            )}
            {hasAPIKey && (
                <TextItem
                    label={intl.formatMessage({id: 'bot.apiKey', defaultMessage: 'API Key'})}
                    type='password'
                    value={props.service.apiKey}
                    onChange={(e) => props.onChange({...props.service, apiKey: e.target.value})}
                />
            )}
            {isOpenAIType && (
                <TextItem
                    label={intl.formatMessage({id: 'bot.orgId', defaultMessage: 'Organization ID'})}
                    value={props.service.orgId}
                    onChange={(e) => props.onChange({...props.service, orgId: e.target.value})}
                />
            )}
            {type === 'asksage' && (
                <>
                    <TextItem
                        label={intl.formatMessage({id: 'bot.username', defaultMessage: 'Username'})}
                        value={props.service.username}
                        onChange={(e) => props.onChange({...props.service, username: e.target.value})}
                    />
                    <TextItem
                        label={intl.formatMessage({id: 'bot.password', defaultMessage: 'Password'})}
                        value={props.service.password}
                        onChange={(e) => props.onChange({...props.service, password: e.target.value})}
                    />
                </>
            )}
            <TextItem
                label={intl.formatMessage({id: 'bot.defaultModel', defaultMessage: 'Default model'})}
                value={props.service.defaultModel}
                onChange={(e) => props.onChange({...props.service, defaultModel: e.target.value})}
            />
            <TextItem
                label={intl.formatMessage({id: 'bot.tokenLimit', defaultMessage: 'Token limit'})}
                value={props.service.tokenLimit.toString()}
                onChange={(e) => props.onChange({...props.service, tokenLimit: parseInt(e.target.value, 10)})}
            />
            {isOpenAIType && (
                <TextItem
                    label={intl.formatMessage({id: 'bot.streamingTimeoutSeconds', defaultMessage: 'Streaming Timeout Seconds'})}
                    value={props.service.streamingTimeoutSeconds?.toString() || '0'}
                    onChange={(e) => props.onChange({...props.service, streamingTimeoutSeconds: parseInt(e.target.value, 10)})}
                />
            )}
        </>
    );
};

const ItemListContainer = styled.div`
	padding: 24px 20px;
	padding-right; 76px;
`;

const Title = styled.div`
	display: flex;
	flex-direction: row;
	align-items: center;
	gap: 8px;
`;

const NameText = styled.div`
	font-size: 14px;
	font-weight: 600;
`;

const ServiceTypeText = styled.div`
	font-size: 14px;
	font-weight: 400;
	color: rgba(var(--center-channel-color-rgb), 0.72);
`;

const Spacer = styled.div`
	flex-grow: 1;
`;

const TrashIcon = styled(TrashCanOutlineIcon)`
	width: 16px;
	height: 16px;
	color: #D24B4E;
`;

const VerticalDivider = styled.div`
	width: 1px;
	border-left: 1px solid rgba(var(--center-channel-color-rgb), 0.16);
	height: 24px;
`;

const BotContainer = styled.div`
	display: flex;
	flex-direction: column;

	border-radius: 4px;
	border: 1px solid rgba(var(--center-channel-color-rgb), 0.12);

	&:hover {
		box-shadow: 0px 2px 3px 0px rgba(0, 0, 0, 0.08);
	}
`;

const HeaderContainer = styled.div`
	display: flex;
	flex-direction: row;
	justify-content: space-between;
	align-items: center;
	gap: 16px;
	padding: 12px 16px 12px 20px;
	border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
	cursor: pointer;
`;

export default Bot;
