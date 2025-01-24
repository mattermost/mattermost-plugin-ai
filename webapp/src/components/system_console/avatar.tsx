// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {ChangeEvent, useEffect, useRef, useState} from 'react';
import styled from 'styled-components';
import {FormattedMessage} from 'react-intl';

//@ts-ignore it exists
import aiIcon from 'src/../../assets/bot_icon.png';

import {getBotProfilePictureUrl} from '@/client';

import {TertiaryButton} from '../assets/buttons';

import {ItemLabel} from './item';

type AvatarItemProps = {
    botusername: string;
    changedAvatar: (image: File) => void;
}

const AvatarItem = (props: AvatarItemProps) => {
    const [icon, setIcon] = useState<string>(aiIcon);
    const hiddenInput = useRef<HTMLInputElement>(null);

    useEffect(() => {
        const getUserIcon = async () => {
            const userIcon = await getBotProfilePictureUrl(props.botusername);
            if (userIcon) {
                setIcon(userIcon);
            }
        };
        getUserIcon();
    }, []);

    const onUploadChange = async (e: ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files[0]) {
            const file = e.target.files[0];

            const reader = new FileReader();
            reader.onload = () => {
                setIcon(URL.createObjectURL(file));
            };
            reader.readAsArrayBuffer(file);
            e.target.value = '';
            props.changedAvatar(file);
        } else {
            setIcon(aiIcon);
        }
    };

    return (
        <>
            <ItemLabel><FormattedMessage defaultMessage='Bot avatar'/></ItemLabel>
            <AvatarSelectorContainer>
                <Avatar src={icon}/>
                <TertiaryButton
                    onClick={() => {
                        if (hiddenInput.current) {
                            hiddenInput.current.click();
                        }
                    }}
                >
                    <HiddenInput
                        ref={hiddenInput}
                        type='file'
                        accept='.jpeg,.jpg,.png,.gif' // From the MM server requirements
                        onChange={onUploadChange}
                    />
                    <FormattedMessage defaultMessage='Upload Image'/>
                </TertiaryButton>
            </AvatarSelectorContainer>
        </>
    );
};

const HiddenInput = styled.input`
	&&& {
		display: none;
	}
`;

const Avatar = styled.img`
	width: 64px;
	height: 64px;
	border-radius: 50%;
`;

const AvatarSelectorContainer = styled.div`
	display: flex;
	flex-direction: row;
	align-items: center;
	gap: 16px;
`;

export default AvatarItem;
