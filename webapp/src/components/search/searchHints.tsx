import React from 'react';
import { FormattedMessage } from 'react-intl';
import styled from 'styled-components';

const SearchHintsContainer = styled.span`
    display: flex;
    padding: 20px 24px;
    color: rgba(var(--center-channel-color-rgb), 0.75);
    i {
        margin-right: 8px;
        color: var(--center-channel-color-56);
    }
`;

const SearchHints = () => (
    <SearchHintsContainer>
        <i className='icon icon-lightbulb-outline'/>
        <FormattedMessage defaultMessage="Copilot searches all channels and messages you have access to"/>
    </SearchHintsContainer>
);

export default SearchHints;

