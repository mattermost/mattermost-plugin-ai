import {useState, useEffect} from 'react';

import {useDispatch, useSelector} from 'react-redux';

import {GlobalState} from '@mattermost/types/store';

import {getAIBots} from '@/client';

import manifest from './manifest';
import {BotsHandler} from './redux';

export interface LLMBot {
    id: string;
    displayName: string;
    username: string;
    lastIconUpdate: number;
    dmChannelID: string;
}

const defaultBotLocalStorageKey = 'defaultBot';

export const useBotlist = () => {
    const bots = useSelector<GlobalState, LLMBot[] | null>((state: any) => state['plugins-' + manifest.id].bots);
    const defaultActiveBotName = localStorage.getItem(defaultBotLocalStorageKey);
    const defaultActiveBot = bots?.find((bot: LLMBot) => bot.username === defaultActiveBotName) || null;
    const [activeBot, setActiveBotState] = useState<LLMBot | null>(defaultActiveBot);
    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);
    const dispatch = useDispatch();

    // Load bots
    useEffect(() => {
        const fetchBots = async () => {
            const fetchedBots = await getAIBots();
            if (!fetchedBots) {
                return;
            }

            dispatch({
                type: BotsHandler,
                bots: fetchedBots,
            });
        };
        if (!bots) {
            fetchBots();
        }
    }, [currentUserId, bots, dispatch]);

    const setActiveBot = (bot: LLMBot) => {
        setActiveBotState(bot);
        localStorage.setItem(defaultBotLocalStorageKey, bot.username);
    };

    return {bots, activeBot, setActiveBot};
};
