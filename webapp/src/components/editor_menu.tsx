import React, {useState} from 'react';
import styled from 'styled-components';

import LoadingSpinner from './assets/loading_spinner';
import IconAI from './assets/icon_ai';
import IconWand from './assets/icon_wand';
import {SubtlePrimaryButton, TertiaryButton, ButtonIcon} from './assets/buttons';
import DotMenu, {DropdownMenuItem} from './dot_menu';
import {doSimplify, doChangeTone} from '../client';

type Props = {
    draft: any, // TODO: Add PostDraft here
    selectedText: string,
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
`

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
`

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
    const [error, setError] = useState('');

    const simplify = async () => {
        setCurrentAction('simplify')
        setGenerating(true)
        let data = {message: ''};
        try {
            data = await doSimplify(draft.message);
        } catch (e) {
            setError("Unable to simplify the text")
        }
        setGenerating(false)
        setProposal(data.message)
    }

    const changeToProfessional = async () => {
        setCurrentAction('change-to-professional')
        setGenerating(true)
        let data = {message: ''};
        try {
            data = await doChangeTone('professional', draft.message);
        } catch (e) {
            setError("Unable to change the tone")
        }
        setGenerating(false)
        setProposal(data.message)
    }

    const regenerate = async () => {
        setProposal('')
        setGenerating(true)
        let data = {message: ''};
        if (currentAction == 'simplify') {
            try {
                data = await doSimplify(draft.message);
            } catch (e) {
                setError("Unable to simplify the text")
            }
        } else if (currentAction == 'change-to-professional') {
            try {
                data = await doChangeTone('professional', draft.message);
            } catch (e) {
                setError("Unable to change the tone")
            }
        }
        setGenerating(false)
        setProposal(data.message)
        setCurrentAction('simplify')
    }

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
                <MenuContent>
                    {generating && <LoadingSpinner/>}
                    {!generating && error &&
                        <div>
                            <div>{error}</div>
                            <MenuContentButtons>
                                <AIPrimaryButton onClick={(e) => {
                                    e.stopPropagation();
                                    e.preventDefault();
                                    setError('');
                                    regenerate();
                                }}>{'Try again'}</AIPrimaryButton>
                                <AISecondaryButton onClick={() => {}}>{'Cancel'}</AISecondaryButton>
                            </MenuContentButtons>
                        </div>
                    }
                    {!error && proposal &&
                        <Proposal
                            text={proposal}
                            onAccept={() => {
                                updateText(proposal)
                            }}
                            onRegenerate={regenerate}
                        />}
                </MenuContent>}
            {!error && !proposal && !generating &&
                <>
                    <DropdownMenuItem onClick={(e: Event) => {
                        e.stopPropagation();
                        e.preventDefault();
                        simplify();
                    }}>
                        <span className='icon'><IconAI/></span>{'Simplify'}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={(e: Event) => {
                        e.stopPropagation();
                        e.preventDefault();
                        changeToProfessional();
                    }}>
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
                <AIPrimaryButton onClick={() => props.onAccept()}><span className='icon'><i className='icon-check'/></span>{'Use this'}</AIPrimaryButton>
                <AISecondaryButton onClick={(e) => {
                    e.stopPropagation();
                    e.preventDefault();
                    props.onRegenerate();
                }}><span className='icon'><i className='icon-refresh'/></span>{'Regenerate'}</AISecondaryButton>
            </MenuContentButtons>
        </div>
    );
}

export default EditorMenu;
