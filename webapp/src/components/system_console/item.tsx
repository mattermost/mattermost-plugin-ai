import React from 'react';
import styled from 'styled-components';
import {FormattedMessage} from 'react-intl';

export const ItemList = styled.div`
	display: grid;
	grid-template-columns: minmax(auto, 275px) 1fr;
	grid-column-gap: 16px;
	grid-row-gap: 24px;
`;

type TextItemProps = {
    label: string,
    value: string,
    type?: string,
    helptext?: string,
    multiline?: boolean,
    placeholder?: string,
    maxLength?: number,
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
};

export const TextItem = (props: TextItemProps) => {
    return (
        <>
            <ItemLabel>{props.label}</ItemLabel>
            <TextFieldContainer>
                <StyledInput
                    as={props.multiline ? 'textarea' : 'input'}
                    value={props.value}
                    type={props.type ? props.type : 'text'}
                    placeholder={props.placeholder ? props.placeholder : props.label}
                    onChange={props.onChange}
                    maxLength={props.maxLength}
                />
                {props.helptext &&
                <HelpText>{props.helptext}</HelpText>
                }
            </TextFieldContainer>
        </>
    );
};

type SelectionItemProps = {
    label: string
    value: string
    onChange: (e: React.ChangeEvent<HTMLSelectElement>) => void
    children: React.ReactNode
};

export const SelectionItem = (props: SelectionItemProps) => {
    return (
        <>
            <ItemLabel>{props.label}</ItemLabel>
            <StyledInput
                as='select'
                value={props.value}
                onChange={props.onChange}
            >
                {props.children}
            </StyledInput>
        </>
    );
};

export const SelectionItemOption = styled.option`
`;

export const ItemLabel = styled.label`
	font-size: 14px;
	font-weight: 600;
	line-height: 20px;
`;

const TextFieldContainer = styled.div`
	display: flex;
	flex-direction: column;
	gap: 8px;
`;

export const HelpText = styled.div`
	font-size: 12px;
	font-weight: 400;
	line-height: 16px;
	color: rgba(var(--center-channel-color-rgb), 0.72);
`;

export const StyledInput = styled.input<{as?: string}>`
	apperance: none;
	display: flex;
	padding: 7px 12px;
	align-items: flex-start;
	border-radius: 2px;
	border: 1px solid rgba(var(--center-channel-color-rgb), 0.16);
	box-shadow: 0px 1px 1px rgba(0, 0, 0, 0.075) inset;
	height: 35px;
	background: white;

	font-size: 14px;
	font-weight: 400;
	line-height: 20px;

	${(props) => props.as === 'textarea' && `
		resize: vertical;
		height: 120px;
	`}

	&:focus {
		border-color: $66afe9;
		box-shadow: inset 0 1px 1px rgba(0, 0, 0, 0.075), 0 0 8px rgba(102, 175, 233, 0.75);
		outline: 0;
	}
`;

type BooleanItemProps = {
    label: React.ReactNode
    value: boolean
    onChange: (to: boolean) => void
    helpText?: string
};

export const BooleanItem = (props: BooleanItemProps) => {
    return (
        <>
            <ItemLabel>{props.label}</ItemLabel>
            <TextFieldContainer>
                <BooleanItemRow>
                    <input
                        type='radio'
                        value='true'
                        checked={props.value}
                        onChange={() => props.onChange(true)}
                    />
                    <FormattedMessage defaultMessage='true'/>
                    <input
                        type='radio'
                        value='false'
                        checked={!props.value}
                        onChange={() => props.onChange(false)}
                    />
                    <FormattedMessage defaultMessage='false'/>
                </BooleanItemRow>
                {props.helpText &&
                <HelpText>{props.helpText}</HelpText>
                }
            </TextFieldContainer>
        </>
    );
};

const BooleanItemRow = styled.div`
	display: flex;
	flex-direction: row;
	gap: 8px;
	align-items: center;
`;
