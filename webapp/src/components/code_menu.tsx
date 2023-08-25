import React, {useState} from 'react';
import styled from 'styled-components';

import {doExplainCode, doSuggestCodeImprovements} from '../client';

import LoadingSpinner from './assets/loading_spinner';
import IconAI from './assets/icon_ai';
import IconWand from './assets/icon_wand';
import {SubtlePrimaryButton, TertiaryButton} from './assets/buttons';
import DotMenu, {DropdownMenuItem} from './dot_menu';

type Props = {
    code: string
}

export const Menu = styled(DotMenu).attrs((props: {$open: boolean}) => ({$open: props.$open}))`
    margin-left: 4px;
    opacity: ${(props) => (props.$open ? '1' : '0')};
    .post-code:hover & {
        opacity: 1;
    }
`;

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

const CodeMenu = ({code}: Props) => {
    const [reply, setReply] = useState<null|string>(null);
    const [generating, setGenerating] = useState(false);
    const [currentAction, setCurrentAction] = useState('');
    const [error, setError] = useState('');
    const [open, setOpen] = useState(false);

    const explainCode = async (e?: Event) => {
        e?.stopPropagation();
        e?.preventDefault();
        setCurrentAction('explain-code');
        setGenerating(true);
        let data = {message: ''};
        try {
            data = await doExplainCode(code);
        } catch (err) {
            setError('Unable to explain the code');
        }
        setGenerating(false);
        setReply(data.message);
    };

    const suggestImprovements = async (e?: Event) => {
        e?.stopPropagation();
        e?.preventDefault();
        setCurrentAction('suggest-code-improvements');
        setGenerating(true);
        let data = {message: ''};
        try {
            data = await doSuggestCodeImprovements(code);
        } catch (err) {
            setError('Unable to suggest code improvements');
        }
        setGenerating(false);
        setReply(data.message);
    };

    const regenerate = async () => {
        setReply('');
        setGenerating(true);
        let data = {message: ''};
        if (currentAction === 'explain-code') {
            try {
                data = await doExplainCode(code);
            } catch (e) {
                setError('Unable to explain the code');
            }
        } else if (currentAction === 'suggest-code-improvements') {
            try {
                data = await doSuggestCodeImprovements(code);
            } catch (e) {
                setError('Unable to change the tone');
            }
        }
        setGenerating(false);
        setReply(data.message);
    };

    return (
        <Menu
            icon={<IconAI/>}
            title='AI Actions'
            $open={open}
            onOpenChange={(isOpen) => {
                setOpen(isOpen);
                setReply('');
                setGenerating(false);
                setCurrentAction('');
                setError('');
            }}
        >
            {(generating || error || reply) &&
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
                    {!error && reply &&
                        <Proposal
                            text={reply}
                            onRegenerate={regenerate}
                        />}
                </MenuContent>}
            {!error && !reply && !generating &&
                <>
                    <DropdownMenuItem onClick={explainCode}>
                        <span className='icon'><IconAI/></span>{'Explain Code'}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={suggestImprovements}>
                        <span className='icon'><IconWand/></span>{'Suggest Improvements'}
                    </DropdownMenuItem>
                </>}
        </Menu>
    );
};

type ProposalProps = {
    text: string,
    onRegenerate: () => void,
}

const Proposal = (props: ProposalProps) => {
    return (
        <div>
            <div>{props.text}</div>
            <MenuContentButtons>
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

export default CodeMenu;
