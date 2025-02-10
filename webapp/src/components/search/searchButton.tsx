import React from 'react';
import styled from 'styled-components';

import IconAI from '../assets/icon_ai';

const SearchButtonContainer = styled.span`
    display: inline-flex;
    align-items: center;
    justify-content: center;
    svg {
        height: 16px;
    }
`;

const SearchButton = () => (
    <SearchButtonContainer>
        <IconAI/>{'Copilot'}
    </SearchButtonContainer>
);

export default SearchButton;
