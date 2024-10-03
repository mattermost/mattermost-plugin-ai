import React from 'react';
import styled from 'styled-components';
import {FormattedMessage, useIntl} from 'react-intl';

import Checkbox from '../checkbox';
import {SelectUser, SelectChannel} from '../select';

import {ChannelAssistanceLevel, UserAssistanceLevel} from './bot';

import {HelpText, ItemLabel} from './item';

const IndentedHelpText = styled(HelpText)`
	margin-left: 24px;
	margin-top: 8px;
`;

const AllowTypes = styled.div`
	margin-left: 24px;
	margin-top: 32px;
	display: grid;
	grid-template-columns: auto 1fr;
	grid-row-gap: 8px;
	grid-column-gap: 8px;
	grid-align-items: center;
`;

const MainContainer = styled.div`
`;

const SelectWrapper = styled.div`
    margin-left: 24px;
    margin-top: 8px;
    width: 90%;
`;

type UserAssistanceLevelProps = {
    label: string;
    level: UserAssistanceLevel;
    onChangeLevel: (level: UserAssistanceLevel) => void;
    userIDs: string[];
    onChangeUserIDs: (userIds: string[]) => void;
};

export const UserAssistanceLevelItem = (props: UserAssistanceLevelProps) => {
    const intl = useIntl();

    return (
        <>
            <ItemLabel>{props.label}</ItemLabel>
            <MainContainer>
                <Checkbox
                    testId='userAssistanceLevelCheckbox'
                    text={intl.formatMessage({defaultMessage: 'Personal assistance'})}
                    checked={props.level !== UserAssistanceLevel.None}
                    onChange={(checked: boolean) => {
                        if (checked) {
                            props.onChangeLevel(UserAssistanceLevel.All);
                        } else {
                            props.onChangeLevel(UserAssistanceLevel.None);
                        }
                    }}
                />
                <IndentedHelpText>
                    <FormattedMessage defaultMessage='Users can chat with this bot and get private assistance about content across all channels the user has access to. Select the users you’d like to allow below.'/>
                </IndentedHelpText>
                {props.level !== UserAssistanceLevel.None &&
                <>
                    <AllowTypes>
                        <input
                            type='radio'
                            value={UserAssistanceLevel.All}
                            checked={props.level === UserAssistanceLevel.All || !props.level}
                            onChange={() => props.onChangeLevel(UserAssistanceLevel.All)}
                        />
                        <FormattedMessage defaultMessage='Allow for all users'/>
                        <input
                            type='radio'
                            value={UserAssistanceLevel.Allow}
                            checked={props.level === UserAssistanceLevel.Allow}
                            onChange={() => props.onChangeLevel(UserAssistanceLevel.Allow)}
                        />
                        <FormattedMessage defaultMessage='Allow for selected users'/>
                        <input
                            type='radio'
                            value={UserAssistanceLevel.Block}
                            checked={props.level === UserAssistanceLevel.Block}
                            onChange={() => props.onChangeLevel(UserAssistanceLevel.Block)}
                        />
                        <FormattedMessage defaultMessage='Block selected users'/>
                    </AllowTypes>
                    {props.level !== UserAssistanceLevel.All &&
                    <SelectWrapper>
                        <SelectUser
                            userIDs={props.userIDs}
                            onChangeUserIDs={props.onChangeUserIDs}
                        />
                    </SelectWrapper>
                    }
                </>
                }
            </MainContainer>
        </>
    );
};

type ChannelAssistanceLevelProps = {
    label: string;
    level: ChannelAssistanceLevel;
    onChangeLevel: (level: ChannelAssistanceLevel) => void;
    channelIDs: string[];
    onChangeChannelIDs: (channelIDs: string[]) => void;
};

export const ChannelAssistanceLevelItem = (props: ChannelAssistanceLevelProps) => {
    const intl = useIntl();

    return (
        <>
            <ItemLabel>{props.label}</ItemLabel>
            <MainContainer>
                <Checkbox
                    testId='channelAssistanceLevelCheckbox'
                    text={intl.formatMessage({defaultMessage: 'Channel-based assistance'})}
                    checked={props.level !== ChannelAssistanceLevel.None}
                    onChange={(checked: boolean) => {
                        if (checked) {
                            props.onChangeLevel(ChannelAssistanceLevel.All);
                        } else {
                            props.onChangeLevel(ChannelAssistanceLevel.None);
                        }
                    }}
                />
                <IndentedHelpText>
                    <FormattedMessage defaultMessage='This bot can be a ‘channel expert’ that can consume the contents of a given channel and provide answers only from content available in the channel. Select the channels you’d like it to allow below.'/>
                </IndentedHelpText>
                {props.level !== ChannelAssistanceLevel.None &&
                <>
                    <AllowTypes>
                        <input
                            type='radio'
                            value={ChannelAssistanceLevel.All}
                            checked={props.level === ChannelAssistanceLevel.All || !props.level}
                            onChange={() => props.onChangeLevel(ChannelAssistanceLevel.All)}
                        />
                        <FormattedMessage defaultMessage='Allow for all channels'/>
                        <input
                            type='radio'
                            value={ChannelAssistanceLevel.Allow}
                            checked={props.level === ChannelAssistanceLevel.Allow}
                            onChange={() => props.onChangeLevel(ChannelAssistanceLevel.Allow)}
                        />
                        <FormattedMessage defaultMessage='Allow for selected channels'/>
                        <input
                            type='radio'
                            value={ChannelAssistanceLevel.Block}
                            checked={props.level === ChannelAssistanceLevel.Block}
                            onChange={() => props.onChangeLevel(ChannelAssistanceLevel.Block)}
                        />
                        <FormattedMessage defaultMessage='Block selected channels'/>
                    </AllowTypes>
                    {props.level !== ChannelAssistanceLevel.All &&
                    <SelectWrapper>
                        <SelectChannel
                            channelIDs={props.channelIDs}
                            onChangeChannelIDs={props.onChangeChannelIDs}
                        />
                    </SelectWrapper>
                    }
                </>
                }
            </MainContainer>
        </>
    );
};

