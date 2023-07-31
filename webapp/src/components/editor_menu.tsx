import React from 'react';

import IconAI from './assets/icon_ai';
import IconWand from './assets/icon_wand';
import DotMenu, {DropdownMenuItem} from './dot_menu';
import {doSimplify, doChangeTone} from '../client';

const EditorMenu = (props: {draft: any, selectedText: string, updateText: (text: string) => void}) => {
    const draft = props.draft;
    const updateText = props.updateText;
    const simplify = async () => {
        let data = await doSimplify(draft.message);
        updateText(data.message);
    }
    const changeToProfessional = async () => {
        let data = await doChangeTone('professional', draft.message);
        updateText(data.message);
    }
    return (
        <DotMenu icon={<IconAI/>} title='AI Actions'>
            <DropdownMenuItem onClick={simplify}><span className='icon'><IconAI/></span>{'Simplify'}</DropdownMenuItem>
            <DropdownMenuItem onClick={changeToProfessional}><span className='icon'><IconWand/></span>{'Make it professional'}</DropdownMenuItem>
        </DotMenu>
    );
};

export default EditorMenu;
