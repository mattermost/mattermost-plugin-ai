// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import styled from 'styled-components';

export const Pill = styled.div`
	background: rgb(var(--semantic-color-info));
	color: white;
	border-radius: 4px;
	font-size: 10px;
	font-weight: 600;
	line-height: 16px;
	padding: 0 4px;
	display: flex;
	align-items: center;
	gap: 6px;
`;

export const DangerPill = styled(Pill)`
	background: rgb(var(--semantic-color-danger));
`;

export const GrayPill = styled(Pill)`
	color: var(--center-channel-color);
	background: rgba(var(--center-channel-color-rgb), 0.08);
`;
