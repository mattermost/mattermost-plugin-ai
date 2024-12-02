import {Client4 as Client4Class, ClientError} from '@mattermost/client';
import {ChannelWithTeamData} from '@mattermost/types/channels';
import {Team} from '@mattermost/types/lib/teams';

import manifest from './manifest';

const Client4 = new Client4Class();

function baseRoute(): string {
    return `/plugins/${manifest.id}`;
}

function postRoute(postid: string): string {
    return `${baseRoute()}/post/${postid}`;
}

function channelRoute(channelid: string): string {
    return `${baseRoute()}/channel/${channelid}`;
}

export async function doReaction(postid: string) {
    const url = `${postRoute(postid)}/react`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doThreadAnalysis(postid: string, analysisType: string, botUsername: string) {
    const url = `${postRoute(postid)}/analyze?botUsername=${botUsername}`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({
            analysis_type: analysisType,
        }),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doTranscribe(postid: string, fileID: string) {
    const url = `${postRoute(postid)}/transcribe/file/${fileID}`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doSummarizeTranscription(postid: string) {
    const url = `${postRoute(postid)}/summarize_transcription`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doStopGenerating(postid: string) {
    const url = `${postRoute(postid)}/stop`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doRegenerate(postid: string) {
    const url = `${postRoute(postid)}/regenerate`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return;
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function doPostbackSummary(postid: string) {
    const url = `${postRoute(postid)}/postback_summary`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function summarizeChannelSince(channelID: string, since: number, prompt: string, botUsername: string) {
    const url = `${channelRoute(channelID)}/since?botUsername=${botUsername}`;
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({
            since,
            preset_prompt: prompt,
        }),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function viewMyChannel(channelID: string) {
    return Client4.viewMyChannel(channelID);
}

export async function getAIDirectChannel(currentUserId: string) {
    const botUser = await Client4.getUserByUsername('ai');
    const dm = await Client4.createDirectChannel([currentUserId, botUser.id]);
    return dm.id;
}

export async function getBotDirectChannel(currentUserId: string, botUserID: string) {
    const dm = await Client4.createDirectChannel([currentUserId, botUserID]);
    return dm.id;
}

export async function getAIThreads() {
    const url = `${baseRoute()}/ai_threads`;
    const response = await fetch(url, Client4.getOptions({
        method: 'GET',
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function getAIBots() {
    const url = `${baseRoute()}/ai_bots`;
    const response = await fetch(url, Client4.getOptions({
        method: 'GET',
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function trackEvent(event: string, source: string, props?: Record<string, string>) {
    const url = `${baseRoute()}/telemetry/track`;
    const userAgent = window.navigator.userAgent;
    const clientType = (userAgent.indexOf('Mattermost') === -1 || userAgent.indexOf('Electron') === -1) ? 'web' : 'desktop';
    const response = await fetch(url, Client4.getOptions({
        method: 'POST',
        body: JSON.stringify({
            event,
            source,
            clientType,
            props: props || {},
        }),
    }));

    if (response.ok) {
        return response.json();
    }

    throw new ClientError(Client4.url, {
        message: '',
        status_code: response.status,
        url,
    });
}

export async function createPost(post: any) {
    const created = await Client4.createPost(post);
    return created;
}

export async function updateRead(userId: string, teamId: string, selectedPostId: string, timestamp: number) {
    Client4.updateThreadReadForUser(userId, teamId, selectedPostId, timestamp);
}

export function getProfilePictureUrl(userId: string, lastIconUpdate: number) {
    return Client4.getProfilePictureUrl(userId, lastIconUpdate);
}

export async function getBotProfilePictureUrl(username: string) {
    const user = await Client4.getUserByUsername(username);
    if (!user || user.id === '') {
        return '';
    }
    return getProfilePictureUrl(user.id, user.last_picture_update);
}

export async function setUserProfilePictureByUsername(username: string, file: File) {
    const user = await Client4.getUserByUsername(username);
    if (!user || user.id === '') {
        return;
    }
    await setUserProfilePicture(user.id, file);
}

export async function setUserProfilePicture(userId: string, file: File) {
    await Client4.uploadProfileImage(userId, file);
}

export async function getAutocompleteAllUsers(name: string) {
    return Client4.autocompleteUsers(name, '', '');
}

export async function getProfilesByIds(userIds: string[]) {
    if (userIds.length === 0) {
        return [];
    }
    return Client4.getProfilesByIds(userIds);
}

export async function searchAllChannels(term: string): Promise<ChannelWithTeamData[]> {
    return Client4.searchAllChannels(term, {
        nonAdminSearch: false,
        public: true,
        private: true,
        include_deleted: false,
        deleted: false,
    }) as Promise<ChannelWithTeamData[]>; // With these paremeters we should always get ChannelWithTeamData[]
}

export async function getChannelById(channelId: string): Promise<ChannelWithTeamData> {
    const channel = await Client4.getChannel(channelId);
    const team = await Client4.getTeam(channel.team_id);
    return {
        ...channel,
        team_name: team.display_name,
        team_display_name: team.display_name,
        team_update_at: team.update_at,
    };
}

export async function getTeamsByIds(teamIds: string[]) {
    if (teamIds.length === 0) {
        return [];
    }
    return Promise.all(teamIds.map((id) => Client4.getTeam(id)));
}

export async function searchTeams(term: string) {
    return Client4.searchTeams(term, {}) as Promise<Team[]>;
}

export function getTeamIconUrl(teamId: string, lastTeamIconUpdate: number) {
    return Client4.getTeamIconUrl(teamId, lastTeamIconUpdate);
}
