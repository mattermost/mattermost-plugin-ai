// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useEffect} from 'react';
import {useSelector} from 'react-redux';
import styled, {keyframes, css} from 'styled-components';
import {executeActions} from '@/client';


import {GlobalState} from '@mattermost/types/store';
import {Channel} from '@mattermost/types/channels';
import {Team} from '@mattermost/types/teams';

import ActionBlock from './action_block';
import Spinner from './spinner';

export type ChannelNamesMap = {
    [name: string]: {
        display_name: string;
        team_name?: string;
    } | Channel;
};

interface Props {
    message: string;
    channelID: string;
    postID: string;
    showCursor?: boolean;
}

const blinkKeyframes = keyframes`
	0% { opacity: 0.48; }
	20% { opacity: 0.48; }
	100% { opacity: 0.12; }
`;

const TextContainer = styled.div<{showCursor?: boolean}>`
	${(props) => props.showCursor && css`
		>ul:last-child>li:last-child>span:not(:has(li))::after,
		>ol:last-child>li:last-child>span:not(:has(li))::after,
		>ul:last-child>li:last-child>span>ul>li:last-child>span:not(:has(li))::after,
		>ol:last-child>li:last-child>span>ul>li:last-child>span:not(:has(li))::after,
		>ul:last-child>li:last-child>span>ol>li:last-child>span:not(:has(li))::after,
		>ol:last-child>li:last-child>span>ol>li:last-child>span:not(:has(li))::after,
		>h1:last-child::after,
		>h2:last-child::after,
		>h3:last-child::after,
		>h4:last-child::after,
		>h5:last-child::after,
		>h6:last-child::after,
		>blockquote:last-child>p::after,
		>p:last-child::after {
			content: '';
			width: 7px;
			height: 16px;
			background: rgba(var(--center-channel-color-rgb), 0.48);
			display: inline-block;
			margin-left: 3px;

			animation: ${blinkKeyframes} 500ms ease-in-out infinite;
		}
	`}
`;

const PostText = (props: Props) => {
    const [actionsBlock, setActionsBlock] = useState<string>('');
    const [loadingBlock, setLoadingBlock] = useState<boolean>(false);

    useEffect(() => {
        const blocks: {[key: string]: string} = {};

        // Find all action blocks in the message
        const regex = /<actions>([\s\S]*?)<\/actions>/g;
        const openRegex = /<actions>/g;
        const closeRegex = /<\/actions>/g;


        let match;

        // Count opening and closing tags
        const openMatches = props.message.match(openRegex)?.length || 0;
        const closeMatches = props.message.match(closeRegex)?.length || 0;

        let block = ''
        // Process complete blocks
        while ((match = regex.exec(props.message)) !== null) {
            block += match[1];
        }

        // Mark incomplete blocks
        if (openMatches > closeMatches) {
            setLoadingBlock(true);
        } else {
            setLoadingBlock(false);
        }

        setActionsBlock(block);
    }, [props.message]);

    const [isExecuting, setIsExecuting] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleExecute = async (actions: any) => {
        try {
            setIsExecuting(true);
            setError(null);
            await executeActions(actions);
        } catch (err: any) {
            setError(err.message || 'Failed to execute actions');
            console.error('Failed to execute actions:', err);
        } finally {
            setIsExecuting(false);
        }
    };
    const channel = useSelector<GlobalState, Channel>((state) => state.entities.channels.channels[props.channelID]);
    const team = useSelector<GlobalState, Team>((state) => state.entities.teams.teams[channel?.team_id]);
    const siteURL = useSelector<GlobalState, string | undefined>((state) => state.entities.general.config.SiteURL);

    // @ts-ignore
    const {formatText, messageHtmlToComponent} = window.PostUtils;

    const markdownOptions = {
        singleline: false,
        mentionHighlight: true,
        atMentions: true,
        team,
        unsafeLinks: true,
        minimumHashtagLength: 1000000000,
        siteURL,
    };

    const messageHtmlToComponentOptions = {
        hasPluginTooltips: true,
        latex: false,
        inlinelatex: false,
        postId: props.postID,
    };

    const preText = (text: string): string => {
        const actionsStart = text.indexOf('<actions>');
        if (actionsStart === -1) {
            return text;
        }
        return text.substring(0, actionsStart);
    };

    const postText = (text: string): string => {
        const lastActionsEnd = text.lastIndexOf('</actions>');
        if (lastActionsEnd === -1) {
            return '';
        }
        return text.substring(lastActionsEnd + '</actions>'.length);
    };

    const preTextString = messageHtmlToComponent(
        formatText(preText(props.message), markdownOptions),
        messageHtmlToComponentOptions,
    );
    const postTextString = messageHtmlToComponent(
        formatText(postText(props.message), markdownOptions),
        messageHtmlToComponentOptions,
    );

    if (!preTextString) {
        return <TextContainer showCursor={props.showCursor}>{<p/>}</TextContainer>;
    }

    return (
        <TextContainer
            data-testid='posttext'
            showCursor={props.showCursor}
        >
            {preTextString}
            {loadingBlock && <Spinner/>}
            {actionsBlock && (
                <ActionBlock
                    content={actionsBlock}
                    onExecute={handleExecute}
                    executionError={error || ''}
                    isExecuting={isExecuting}
                    channelID={props.channelID}
                />
            )}
            {postTextString}
        </TextContainer>
    );
};

export default PostText;
