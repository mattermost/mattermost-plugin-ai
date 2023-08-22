import React, {useState} from 'react';
import styled from 'styled-components';

import {Draft} from '@mattermost/types/drafts';

import {doSimplify, doChangeTone, doAskAiChangeText} from '../client';

import LoadingSpinner from './assets/loading_spinner';
import IconAI from './assets/icon_ai';
import IconWand from './assets/icon_wand';
import {SubtlePrimaryButton, TertiaryButton} from './assets/buttons';
import DotMenu, {DropdownMenuItem} from './dot_menu';
import AskAiInput from './ask_ai_input';

type Props = {
    draft: Draft,
    getSelectedText: () => {start: number, end: number},
    updateText: (text: string) => void
}

export const MenuContent = styled.div`
 && {
     display: flex;
     align-items: center;
     justify-content: center;
     padding: 10px 20px;
     max-width: 500px;
}
`;

export const AIPrimaryButton = styled(SubtlePrimaryButton)`
    height: 24px;
    padding: 0 10px;
    margin-right: 10px;
`;

export const AISecondaryButton = styled(TertiaryButton)`
    height: 24px;
    padding: 0 10px;
    margin-right: 10px;
    background: rgba(var(--center-channel-color-rgb), 0.08);
    color: rgba(var(--center-channel-color-rgb), 0.72);
    fill: rgba(var(--center-channel-color-rgb), 0.72);
    &:hover {
        background: rgba(var(--center-channel-color-rgb), 0.12);
        color: rgba(var(--center-channel-color-rgb), 0.76);
        fill: rgba(var(--center-channel-color-rgb), 0.76);
    }
`;

export const MenuContentButtons = styled.div`
 && {
     display: inline-flex
     align-items: center;
     justify-content: center;
     margin-top: 10px;
}
`;

const EditorMenu = (props: Props) => {
    const draft = props.draft;
    const updateText = props.updateText;
    const [proposal, setProposal] = useState<null|string>(null);
    const [generating, setGenerating] = useState(false);
    const [currentAction, setCurrentAction] = useState('');
    const [lastChangeAsk, setLastChangeAsk] = useState('');
    const [error, setError] = useState('');

    const simplify = async (e?: Event) => {
        e?.stopPropagation();
        e?.preventDefault();
        setCurrentAction('simplify');
        setGenerating(true);
        const {start, end} = props.getSelectedText();
        let text = draft.message;
        if (start < end) {
            text = draft.message.substring(start, end);
        }
        let data = {message: ''};
        try {
            data = await doSimplify(text);
        } catch (err) {
            setError('Unable to simplify the text');
        }
        setGenerating(false);
        setProposal(data.message);
    };

    const changeToProfessional = async (e?: Event) => {
        e?.stopPropagation();
        e?.preventDefault();
        setCurrentAction('change-to-professional');
        setGenerating(true);
        const {start, end} = props.getSelectedText();
        let text = draft.message;
        if (start < end) {
            text = draft.message.substring(start, end);
        }
        let data = {message: ''};
        try {
            data = await doChangeTone('professional', text);
        } catch (err) {
            setError('Unable to change the tone');
        }
        setGenerating(false);
        setProposal(data.message);
    };

    const askAiChangeText = async (ask: string) => {
        setCurrentAction('ask-ai-change-text');
        setLastChangeAsk(ask);
        setGenerating(true);
        const {start, end} = props.getSelectedText();
        let text = draft.message;
        if (start < end) {
            text = draft.message.substring(start, end);
        }
        let data = {message: ''};
        try {
            data = await doAskAiChangeText(ask, text);
        } catch (e) {
            setError('Unable to change the text');
        }
        setGenerating(false);
        setProposal(data.message);
    };

    const regenerate = async () => {
        setProposal('');
        setGenerating(true);
        const {start, end} = props.getSelectedText();
        let text = draft.message;
        if (start < end) {
            text = draft.message.substring(start, end);
        }
        let data = {message: ''};
        if (currentAction === 'simplify') {
            try {
                data = await doSimplify(text);
            } catch (e) {
                setError('Unable to simplify the text');
            }
        } else if (currentAction === 'change-to-professional') {
            try {
                data = await doChangeTone('professional', text);
            } catch (e) {
                setError('Unable to change the tone');
            }
        } else if (currentAction === 'ask-ai-change-text') {
            try {
                data = await doAskAiChangeText(lastChangeAsk, text);
            } catch (e) {
                setError('Unable to change the text');
            }
        }
        setGenerating(false);
        setProposal(data.message);
        setCurrentAction('simplify');
    };

    return (
        <DotMenu
            icon={<IconAI/>}
            title='AI Actions'
            onOpenChange={() => {
                setProposal('');
                setGenerating(false);
                setCurrentAction('');
                setError('');
            }}
        >
            {(generating || error || proposal) &&
                <MenuContent
                    onClick={(e) => {
                        if (!(e.target as HTMLElement).classList.contains('ai-error-cancel') && !(e.target as HTMLElement).classList.contains('ai-use-it-button')) {
                            e.stopPropagation();
                            e.preventDefault();
                        }
                    }}
                >
                    {generating && <LoadingSpinner/>}
                    {!generating && error &&
                        <div>
                            <div>{error}</div>
                            <MenuContentButtons>
                                <AIPrimaryButton
                                    onClick={() => {
                                        setError('');
                                        regenerate();
                                    }}
                                >{'Try again'}</AIPrimaryButton>
                                <AISecondaryButton className='ai-error-cancel'>{'Cancel'}</AISecondaryButton>
                            </MenuContentButtons>
                        </div>
                    }
                    {!error && proposal &&
                        <Proposal
                            text={proposal}
                            onAccept={() => {
                                const {start, end} = props.getSelectedText();
                                let prefix = '';
                                let suffix = '';
                                if (start < end) {
                                    prefix = draft.message.substring(0, start);
                                    suffix = draft.message.substring(end);
                                }
                                updateText(prefix + proposal + suffix);
                            }}
                            onRegenerate={regenerate}
                        />}
                </MenuContent>}
            {!error && !proposal && !generating &&
                <>
                    <AskAiInput
                        placeholder='Ask AI to edit selection...'
                        onRun={(text: string) => {
                            askAiChangeText(text);
                        }}
                    />
                    <DropdownMenuItem onClick={simplify}>
                        <span className='icon'><IconAI/></span>{'Simplify'}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={changeToProfessional}>
                        <span className='icon'><IconWand/></span>{'Make it professional'}
                    </DropdownMenuItem>
                </>}
        </DotMenu>
    );
};

type ProposalProps = {
    text: string,
    onAccept: () => void,
    onRegenerate: () => void,
}

const Proposal = (props: ProposalProps) => {
    return (
        <div>
            <div>{props.text}</div>
            <MenuContentButtons>
                <AIPrimaryButton
                    onClick={() => props.onAccept()}
                    className='ai-use-it-button'
                ><span className='icon'><i className='icon-check'/></span>{'Use this'}</AIPrimaryButton>
                <AISecondaryButton
                    onClick={(e) => {
                        e.stopPropagation();
                        e.preventDefault();
                        props.onRegenerate();
                    }}
                ><span className='icon'><i className='icon-refresh'/></span>{'Regenerate'}</AISecondaryButton>
            </MenuContentButtons>
        </div>
    );
};

export default EditorMenu;
