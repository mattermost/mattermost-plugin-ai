// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import styled from 'styled-components';

import IconAI from './assets/icon_ai';

const Container = styled.div`
	display: flex;
	position: relative;
	border: 1px solid #ddd;
	border-radius: 5px;
	margin: 5px 10px;
	padding: 5px 10px;
`;

const Input = styled.input`
	border: none;
	backgroundColor: transparent;
`;

const Icon = styled.span`
	display: inline-flex;
	align-items: center;
	justify-content: center;
	font-size: 10px;
	padding-right: 5px;
`;

type Props = {
    placeholder: string;
    onRun: (value: string) => void;
}

export default function AskAiInput(props: Props) {
    const [value, setValue] = useState('');
    return (
        <Container onClick={(e) => e.stopPropagation()}>
            <Icon><IconAI/></Icon>
            <Input
                type='text'
                placeholder={props.placeholder}
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                        e.preventDefault();
                        props.onRun(value);
                    }
                }}
            />
        </Container>
    );
}

