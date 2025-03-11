// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useSelector, useDispatch} from 'react-redux';
import styled from 'styled-components';

import {GlobalState} from '@mattermost/types/store';

import {PostMessagePreview} from '@/mm_webapp';
import {getPost, getProfilesByIds} from '@/client';

const MessagePreviewWrapper = styled.div`
    margin-left: 20px;
    margin-top: 4px;
`;

interface Props {
    postId: string;
    userId: string;
    channelId: string;
    content: string;
}

export const PostPreview: React.FC<Props> = ({postId, userId, channelId, content}) => {
    const dispatch = useDispatch();
    const channel = useSelector((state: GlobalState) => state.entities.channels.channels[channelId]);
    const team = useSelector((state: GlobalState) => state.entities.teams.teams[channel?.team_id || '']);
    const teamName = team?.name || '';

    useEffect(() => {
        async function fetchData() {
            const [post, profiles] = await Promise.all([
                getPost(postId),
                getProfilesByIds([userId]),
            ]);

            // Store post in Redux
            dispatch({
                type: 'RECEIVED_POST',
                data: post,
            });

            // Store profiles in Redux
            const profilesById = profiles.reduce<Record<string, any>>((acc, profile) => {
                acc[profile.id] = profile;
                return acc;
            }, {});

            dispatch({
                type: 'RECEIVED_PROFILES',
                data: profilesById,
            });
        }

        fetchData();
    }, [dispatch, postId, userId]);

    return (
        <MessagePreviewWrapper>
            <PostMessagePreview
                metadata={{
                    channel_display_name: null,
                    channel_id: channelId,
                    channel_type: channel?.type,
                    post_id: postId,
                    team_name: teamName,
                    post: {
                        message: content,
                        user_id: userId,
                    },
                }}
            />
        </MessagePreviewWrapper>
    );
};
