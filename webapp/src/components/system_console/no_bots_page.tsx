// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {PlusIcon} from '@mattermost/compass-icons/components';
import React from 'react';
import styled from 'styled-components';

import {PrimaryButton} from 'src/components/assets/buttons';
import SparklesGraphic from 'src/components/assets/sparkles_graphic';

import {PanelContainer} from './panel';

type Props = {
    onAddBotPressed: () => void;
};

const NoBotsPage = (props: Props) => {
    return (
        <StyledPanelContainer>
            <SparklesGraphic/>
            <Title>{'No AI bots added yet'}</Title>
            <Subtitle>{'To get started with Copilot, add an AI bot'}</Subtitle>
            <PrimaryButton onClick={props.onAddBotPressed}>
                <StyledPlusIcon/>
                {'Add an AI Bot'}
            </PrimaryButton>
        </StyledPanelContainer>
    );
};

const StyledPlusIcon = styled(PlusIcon)`
	margin-right: 8px;
	width: 18px;
	height: 18px;
`;

const StyledPanelContainer = styled(PanelContainer)`
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 16px;
	padding-bottom: 56px;
`;

const Title = styled.div`
	font-size: 20px;
	font-weight: 600;
	font-family: Metropolis;
	line-height: 28px;
`;

const Subtitle = styled.div`
	font-size: 14px;
	font-weight: 400;
	line-height: 20px;
`;

export default NoBotsPage;
