import React from 'react';
import styled from 'styled-components';
import {PlusIcon} from '@mattermost/compass-icons/components';

import {TertiaryButton} from '../assets/buttons';

import {useIsMultiLLMLicensed} from '@/license';

import Bot, {LLMBotConfig} from './bot';
import EnterpriseChip from './enterprise_chip';

const defaultNewBot = {
    id: '',
    name: '',
    displayName: '',
    customInstructions: '',
    service: {
        id: '',
        type: 'openai',
        apiKey: '',
        apiURL: '',
        orgId: '',
        defaultModel: '',
        username: '',
        password: '',
        tokenLimit: 0,
    },
};

export const firstNewBot = {
    ...defaultNewBot,
    name: 'ai',
    displayName: 'Copilot',
};

type Props = {
    bots: LLMBotConfig[]
    onChange: (bots: LLMBotConfig[]) => void
    botChangedAvatar: (bot: LLMBotConfig, image: File) => void
}

const Bots = (props: Props) => {
    const multiLLMLicensed = useIsMultiLLMLicensed();
    const licenceAddDisabled = !multiLLMLicensed && props.bots.length > 0;

    const addNewBot = () => {
        const id = Math.random().toString(36).substring(2, 22);
        if (props.bots.length === 0) {
            // Suggest the '@ai' and 'Copilot' name for the first bot
            props.onChange([{
                ...firstNewBot,
                id,
            }]);
        } else {
            props.onChange([...props.bots, {
                ...defaultNewBot,
                id,
            }]);
        }
    };

    const onChange = (newBot: LLMBotConfig) => {
        props.onChange(props.bots.map((b) => (b.id === newBot.id ? newBot : b)));
    };

    const onDelete = (id: string) => {
        props.onChange(props.bots.filter((b) => b.id !== id));
    };

    return (
        <>
            <BotsList>
                {props.bots.map((bot) => (
                    <Bot
                        key={bot.id}
                        bot={bot}
                        onChange={onChange}
                        onDelete={() => onDelete(bot.id)}
                        changedAvatar={(image: File) => props.botChangedAvatar(bot, image)}
                    />
                ))}
            </BotsList>
            <EnterpriseChipContainer>
                <TertiaryButton
                    onClick={addNewBot}
                    disabled={licenceAddDisabled}
                >
                    <PlusAIServiceIcon/>
                    {'Add an AI Bot'}
                </TertiaryButton>
                {licenceAddDisabled && (
                    <EnterpriseChip
                        text={'Use multiple AI bots on Enterprise plans'}
                        subtext={'Multiple AI services is available on Enterprise plans'}
                    />
                )}
            </EnterpriseChipContainer>
        </>
    );
};

const EnterpriseChipContainer = styled.div`
	display: flex;
	flex-direction: row;
	align-items: center;
	gap: 8px;
`;

const PlusAIServiceIcon = styled(PlusIcon)`
	width: 18px;
	height: 18px;
	margin-right: 8px;
`;

const BotsList = styled.div`
	display: flex;
	flex-direction: column;
	gap: 12px;

	padding-bottom: 24px;
`;

export default Bots;
