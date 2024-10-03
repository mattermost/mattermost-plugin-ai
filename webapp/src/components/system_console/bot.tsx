import React, {useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage, useIntl} from 'react-intl';

import {TrashCanOutlineIcon, ChevronDownIcon, AlertOutlineIcon, ChevronUpIcon} from '@mattermost/compass-icons/components';

import IconAI from '../assets/icon_ai';
import {DangerPill, Pill} from '../pill';

import {ButtonIcon} from '../assets/buttons';

import {BooleanItem, ItemList, SelectionItem, SelectionItemOption, TextItem} from './item';
import AvatarItem from './avatar';
import {ChannelAssistanceLevelItem, UserAssistanceLevelItem} from './assistance_level_item';

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

export enum ChannelAssistanceLevel {
    All = 0,
    Allow,
    Block,
    None,
}

export enum UserAssistanceLevel {
    All = 0,
    Allow,
    Block,
    None,
}

export type LLMBotConfig = {
    id: string
    name: string
    displayName: string
    service: LLMService
    customInstructions: string
    enableVision: boolean
    disableTools: boolean
    channelAssistanceLevel: ChannelAssistanceLevel
    channelIDs: string[]
    userAssistanceLevel: UserAssistanceLevel
    userIDs: string[]
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
    ['azure', 'Azure'],
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
		(props.bot.service.type !== 'asksage' && props.bot.service.type !== 'openaicompatible' && props.bot.service.type !== 'azure' && props.bot.service.apiKey === '') ||
		((props.bot.service.type === 'openaicompatible' || props.bot.service.type === 'azure') && props.bot.service.apiURL === '');

    const invalidUsername = props.bot.name !== '' && (!(/^[a-z0-9.\-_]+$/).test(props.bot.name) || !(/[a-z]/).test(props.bot.name.charAt(0)));
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
                        <FormattedMessage defaultMessage='Missing information'/>
                    </DangerPill>
                )}
                {invalidUsername && (
                    <DangerPill>
                        <AlertOutlineIcon/>
                        <FormattedMessage defaultMessage='Invalid Username'/>
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
                            label={intl.formatMessage({defaultMessage: 'Display name'})}
                            value={props.bot.displayName}
                            onChange={(e) => props.onChange({...props.bot, displayName: e.target.value})}
                        />
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'Bot Username'})}
                            helptext={intl.formatMessage({defaultMessage: 'Team members can mention this bot with this username'})}
                            maxLength={22}
                            value={props.bot.name}
                            onChange={(e) => props.onChange({...props.bot, name: e.target.value})}
                        />
                        <AvatarItem
                            botusername={props.bot.name}
                            changedAvatar={props.changedAvatar}
                        />
                        <SelectionItem
                            label={intl.formatMessage({defaultMessage: 'Service'})}
                            value={props.bot.service.type}
                            onChange={(e) => props.onChange({...props.bot, service: {...props.bot.service, type: e.target.value}})}
                        >
                            <SelectionItemOption value='openai'>{'OpenAI'}</SelectionItemOption>
                            <SelectionItemOption value='openaicompatible'>{'OpenAI Compatible'}</SelectionItemOption>
                            <SelectionItemOption value='azure'>{'Azure'}</SelectionItemOption>
                            <SelectionItemOption value='anthropic'>{'Anthropic'}</SelectionItemOption>
                            <SelectionItemOption value='asksage'>{'Ask Sage (Experimental)'}</SelectionItemOption>
                        </SelectionItem>
                        <ServiceItem
                            service={props.bot.service}
                            onChange={(service) => props.onChange({...props.bot, service})}
                        />
                        <TextItem
                            label={intl.formatMessage({defaultMessage: 'Custom instructions'})}
                            placeholder={intl.formatMessage({defaultMessage: 'How would you like the AI to respond?'})}
                            multiline={true}
                            value={props.bot.customInstructions}
                            onChange={(e) => props.onChange({...props.bot, customInstructions: e.target.value})}
                        />
                        { (props.bot.service.type === 'openai' || props.bot.service.type === 'openaicompatible' || props.bot.service.type === 'azure') && (
                            <>
                                <BooleanItem
                                    label={
                                        <Horizontal>
                                            <FormattedMessage defaultMessage='Enable Vision'/>
                                            <Pill><FormattedMessage defaultMessage='BETA'/></Pill>
                                        </Horizontal>
                                    }
                                    value={props.bot.enableVision}
                                    onChange={(to: boolean) => props.onChange({...props.bot, enableVision: to})}
                                    helpText={intl.formatMessage({defaultMessage: 'Enable Vision to allow the bot to process images. Requires a compatible model.'})}
                                />
                                <BooleanItem
                                    label={
                                        <FormattedMessage defaultMessage='Disable Tools'/>
                                    }
                                    value={props.bot.disableTools}
                                    onChange={(to: boolean) => props.onChange({...props.bot, disableTools: to})}
                                    helpText={intl.formatMessage({defaultMessage: 'By default some tool use is enabled to allow for features such as integrations with JIRA. Disabling this allows use of models that do not support or are not very good at tool use. Some features will not work without tools.'})}
                                />
                            </>
                        )}
                        <ChannelAssistanceLevelItem
                            label={intl.formatMessage({defaultMessage: 'Channel Assistance Level'})}
                            level={props.bot.channelAssistanceLevel ?? ChannelAssistanceLevel.All}
                            onChangeLevel={(to: ChannelAssistanceLevel) => props.onChange({...props.bot, channelAssistanceLevel: to})}
                            channelIDs={props.bot.channelIDs ?? []}
                            onChangeChannelIDs={(channels: string[]) => props.onChange({...props.bot, channelIDs: channels})}
                        />
                        <UserAssistanceLevelItem
                            label={intl.formatMessage({defaultMessage: 'User Assistance Level'})}
                            level={props.bot.userAssistanceLevel ?? ChannelAssistanceLevel.All}
                            onChangeLevel={(to: UserAssistanceLevel) => props.onChange({...props.bot, userAssistanceLevel: to})}
                            userIDs={props.bot.userIDs ?? []}
                            onChangeUserIDs={(users: string[]) => props.onChange({...props.bot, userIDs: users})}
                        />

                    </ItemList>
                </ItemListContainer>
            )}
        </BotContainer>
    );
};

const Horizontal = styled.div`
	display: flex;
	flex-direction: row;
	align-items: center;
	gap: 8px;
`;

type ServiceItemProps = {
    service: LLMService
    onChange: (service: LLMService) => void
}

const ServiceItem = (props: ServiceItemProps) => {
    const type = props.service.type;
    const intl = useIntl();
    const hasAPIKey = type !== 'asksage';
    const isOpenAIType = type === 'openai' || type === 'openaicompatible' || type === 'azure';
    return (
        <>
            {(type === 'openaicompatible' || type === 'azure') && (
                <TextItem
                    label={intl.formatMessage({defaultMessage: 'API URL'})}
                    value={props.service.apiURL}
                    onChange={(e) => props.onChange({...props.service, apiURL: e.target.value})}
                />
            )}
            {hasAPIKey && (
                <TextItem
                    label={intl.formatMessage({defaultMessage: 'API Key'})}
                    type='password'
                    value={props.service.apiKey}
                    onChange={(e) => props.onChange({...props.service, apiKey: e.target.value})}
                />
            )}
            {isOpenAIType && (
                <TextItem
                    label={intl.formatMessage({defaultMessage: 'Organization ID'})}
                    value={props.service.orgId}
                    onChange={(e) => props.onChange({...props.service, orgId: e.target.value})}
                />
            )}
            {type === 'asksage' && (
                <>
                    <TextItem
                        label={intl.formatMessage({defaultMessage: 'Username'})}
                        value={props.service.username}
                        onChange={(e) => props.onChange({...props.service, username: e.target.value})}
                    />
                    <TextItem
                        label={intl.formatMessage({defaultMessage: 'Password'})}
                        value={props.service.password}
                        onChange={(e) => props.onChange({...props.service, password: e.target.value})}
                    />
                </>
            )}
            <TextItem
                label={intl.formatMessage({defaultMessage: 'Default model'})}
                value={props.service.defaultModel}
                onChange={(e) => props.onChange({...props.service, defaultModel: e.target.value})}
            />
            <TextItem
                label={intl.formatMessage({defaultMessage: 'Token limit'})}
                value={props.service.tokenLimit.toString()}
                onChange={(e) => props.onChange({...props.service, tokenLimit: parseInt(e.target.value, 10)})}
            />
            {isOpenAIType && (
                <TextItem
                    label={intl.formatMessage({defaultMessage: 'Streaming Timeout Seconds'})}
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
