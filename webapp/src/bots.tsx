import {useState, useEffect} from 'react';

import {useSelector} from 'react-redux';

import {GlobalState} from '@mattermost/types/store';

import {getAIBots} from '@/client';

export interface LLMBot {
    id: string;
    displayName: string;
    username: string;
    lastIconUpdate: number;
    dmChannelID: string;
}

const defaultBotLocalStorageKey = 'defaultBot';

export const useBotlist = () => {
    const [bots, setBots] = useState<LLMBot[] | null>(null);
    const [activeBot, setActiveBotState] = useState<LLMBot | null>(null);
    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);

    // Load bots
    useEffect(() => {
        const fetchBots = async () => {
            const fetchedBots = await getAIBots();
            if (!fetchedBots) {
                return;
            }

            // Set default bot to the one in local storage otherwise default to the first bot (which should be the server default)
            let newActiveBot = fetchedBots[0];
            if (fetchedBots.length > 1) {
                const defaultBotName = localStorage.getItem(defaultBotLocalStorageKey);
                const defaultBot = fetchedBots.find((bot: LLMBot) => bot.username === defaultBotName);
                if (defaultBot) {
                    newActiveBot = defaultBot;
                }
            }

            setBots(fetchedBots);
            setActiveBotState(newActiveBot);
        };
        fetchBots();
    }, [currentUserId]);

    const setActiveBot = (bot: LLMBot) => {
        setActiveBotState(bot);
        localStorage.setItem(defaultBotLocalStorageKey, bot.username);
    };

    return {bots, activeBot, setActiveBot};
};
