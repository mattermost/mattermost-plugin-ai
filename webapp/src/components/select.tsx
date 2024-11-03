import React, {useState, useEffect} from 'react';
import styled from 'styled-components';
import AsyncSelect from 'react-select/async';
import {StylesConfig, MultiValue} from 'react-select';
import {LockIcon, GlobeIcon} from '@mattermost/compass-icons/components';

import {UserProfile} from '@mattermost/types/users';
import {ChannelType, ChannelWithTeamData} from '@mattermost/types/channels';

import {getAutocompleteAllUsers, getChannelById, getProfilePictureUrl, getProfilesByIds, searchAllChannels} from '../client';

type Option = {
    value: string;
    label: string;
};

type UserOption = Option & {
    avatar: string;
};

type ChannelOption = Option & {
    type: ChannelType;
    teamName: string;
};

type SelectProps<T extends Option> = {
    ids: string[];
    onChangeIds: (ids: string[]) => void;
    fetchSelected: (ids: string[]) => Promise<T[]>;
    fetchOptions: (inputValue: string) => Promise<T[]>;
    formatOptionLabel: (option: T) => React.ReactNode;
    placeholder: string;
};

const LabelContainer = styled.div`
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 14px;
`;

const UserAvatar = styled.img`
    width: 20px;
    height: 20px;
    border-radius: 50%;
    margin-right: 0;
`;

const ChannelName = styled.span`
    font-weight: 600;
`;

const TeamName = styled.span`
    color: rgba(var(--center-channel-color-rgb), 0.56);
    font-weight: normal;
    margin-left: 5px;
`;

const ChannelIcon = styled.span`
    margin-right: 8px;
    display: flex;
    align-items: center;
    color: rgba(var(--center-channel-color-rgb), 0.56);
`;

function SelectComponent<T extends Option>(props: SelectProps<T>) {
    const [selectedOptions, setSelectedOptions] = useState<T[]>([]);

    useEffect(() => {
        const loadSelectedOptions = async () => {
            if (props.ids.length > 0) {
                const options = await props.fetchSelected(props.ids);
                setSelectedOptions(options);
            } else {
                setSelectedOptions([]);
            }
        };
        loadSelectedOptions();
    }, [props.ids, props.fetchSelected]);

    const handleChange = (newValue: MultiValue<T>) => {
        props.onChangeIds(newValue.map((option) => option.value));
    };

    const loadOptions = async (inputValue: string) => {
        return props.fetchOptions(inputValue);
    };

    const selectStyles: StylesConfig<T, true> = {
        multiValue: (base) => ({
            ...base,
            backgroundColor: 'rgba(var(--center-channel-color-rgb), 0.08)',
            borderRadius: '16px',
        }),
        multiValueRemove: (base) => ({
            ...base,
            color: 'rgba(var(--center-channel-color-rgb), 0.56)',
            cursor: 'pointer',
            borderRadius: '50%',
            padding: '0',
            margin: '5px',
            '&:hover': {
                backgroundColor: 'rgba(var(--center-channel-color-rgb), 0.08)',
                color: 'rgba(var(--center-channel-color-rgb), 0.72)',
            },
        }),
    };

    return (
        <AsyncSelect<T, true>
            isMulti={true}
            isClearable={false}
            value={selectedOptions}
            onChange={handleChange}
            loadOptions={loadOptions}
            formatOptionLabel={props.formatOptionLabel}
            placeholder={props.placeholder}
            styles={selectStyles}
            defaultOptions={true}
        />
    );
}

type SelectUserProps = {
    userIDs: string[];
    onChangeUserIDs: (userIds: string[]) => void;
};

export const SelectUser = (props: SelectUserProps) => {
    const fetchSelectedUsers = async (ids: string[]) => {
        const users = await getProfilesByIds(ids);
        return users.map((user: UserProfile) => ({
            value: user.id,
            label: user.username,
            avatar: getProfilePictureUrl(user.id, user.last_picture_update),
        }));
    };

    const fetchUsers = async (inputValue: string) => {
        const initialUsers = await getAutocompleteAllUsers(inputValue);
        return initialUsers.users.
            filter((user: UserProfile) => !user.is_bot). // Remove bot users
            map((user: UserProfile) => ({
                value: user.id,
                label: user.username,
                avatar: getProfilePictureUrl(user.id, user.last_picture_update),
            }));
    };

    const formatUserOptionLabel = (option: UserOption) => (
        <LabelContainer>
            <UserAvatar src={option.avatar}/>
            {option.label}
        </LabelContainer>
    );

    return (
        <SelectComponent<UserOption>
            ids={props.userIDs}
            onChangeIds={props.onChangeUserIDs}
            fetchSelected={fetchSelectedUsers}
            fetchOptions={fetchUsers}
            formatOptionLabel={formatUserOptionLabel}
            placeholder='Search for people'
        />
    );
};

type SelectChannelProps = {
    channelIDs: string[];
    onChangeChannelIDs: (channelIds: string[]) => void;
};

export const SelectChannel = (props: SelectChannelProps) => {
    const fetchSelectedChannels = async (ids: string[]) => {
        const channels = await Promise.all(ids.map((id) => getChannelById(id)));
        return channels.map((channel: ChannelWithTeamData) => ({
            value: channel.id,
            label: channel.display_name,
            type: channel.type,
            teamName: channel.team_display_name,
        }));
    };

    const fetchChannels = async (inputValue: string) => {
        const channels = await searchAllChannels(inputValue);
        return channels.map((channel: ChannelWithTeamData) => ({
            value: channel.id,
            label: channel.display_name,
            type: channel.type,
            teamName: channel.team_display_name,
        }));
    };

    const formatChannelOptionLabel = (option: ChannelOption) => (
        <LabelContainer>
            <ChannelIcon>
                {option.type === 'O' ? <GlobeIcon size={16}/> : <LockIcon size={16}/>}
            </ChannelIcon>
            <ChannelName>{option.label}</ChannelName>
            <TeamName>
                {'('}{option.teamName}{')'}
            </TeamName>
        </LabelContainer>
    );

    return (
        <SelectComponent<ChannelOption>
            ids={props.channelIDs}
            onChangeIds={props.onChangeChannelIDs}
            fetchSelected={fetchSelectedChannels}
            fetchOptions={fetchChannels}
            formatOptionLabel={formatChannelOptionLabel}
            placeholder='Search for channels'
        />
    );
};
