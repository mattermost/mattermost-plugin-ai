import React from 'react';
import {FormattedMessage} from 'react-intl';
import styled from 'styled-components';
import {LightbulbOutlineIcon} from '@mattermost/compass-icons/components';

const DropdownMenuItemInfo = styled.div`
	display: flex;
	align-items: flex-start;
	gap: 8px;

	font-size: 12px;
	font-weight: 400;
	line-height: 16px;
	color: rgba(var(--center-channel-color-rgb), 0.72);

	max-width: 240px;
	padding: 8px 16px;
`;

const LightbulbOutlineIconStyled = styled(LightbulbOutlineIcon)`
	min-width: 22px;
	min-height: 22px;

	padding: 4px;

	color: rgba(var(--center-channel-color-rgb), 0.56);
	background: rgba(var(--center-channel-color-rgb), 0.08);
	border-radius: 16px;
`;

export const Divider = styled.div`
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.08);
    margin-top: 8px;
    margin-bottom: 8px;
`;

export const DropdownInfoOnlyVisibleToYou = () => {
    return (
        <DropdownMenuItemInfo>
            <LightbulbOutlineIconStyled/>
            <FormattedMessage defaultMessage='Copilot posts responses in the right panel which will only be visible to you.'/>
        </DropdownMenuItemInfo>
    );
};
