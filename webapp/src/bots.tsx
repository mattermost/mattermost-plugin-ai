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

export const useBotlist = () => {
    const [bots, setBots] = useState<LLMBot[] | null>(null);
    const [activeBot, setActiveBot] = useState<LLMBot | null>(null);
    const currentUserId = useSelector<GlobalState, string>((state) => state.entities.users.currentUserId);

    // Load bots
    useEffect(() => {
        const fetchBots = async () => {
            const fetchedBots = await getAIBots();
            if (!fetchedBots) {
                return;
            }

            // The default bot should always be the first one.
            const newActiveBot = fetchedBots[0];
            setBots(fetchedBots);
            setActiveBot(newActiveBot);
        };
        fetchBots();
    }, [currentUserId]);

    return {bots, activeBot, setActiveBot};
};
