import React from 'react';
import styled from 'styled-components';

import IconAI from '../assets/icon_ai';

const SearchSuggestionsContainer = styled.span`
    display: flex;
    flex-direction: column;
    justify-content: center;
`;

const SearchSuggestion = styled.div`
    display: flex;
    padding: 8px 24px;
    align-items: center;
    i {
        color: rgba(var(--center-channel-color-rgb), 0.56);
    }
    svg {
        color: rgba(var(--center-channel-color-rgb), 0.56);
        margin-right: 8px;
    }
`;

const SuggestionsHeader = styled.div`
    margin-top: 16px;
    padding: 8px 24px;
    color: rgba(var(--center-channel-color-rgb), 0.56);
    font-size: 12px;
    line-height: 16px;
    font-weight: 600;
    text-transform: uppercase;
`;

const SearchSuggestions = () => (
    <SearchSuggestionsContainer>
        <SuggestionsHeader>{'Recent'}</SuggestionsHeader>
        <SearchSuggestion>
            <i className='icon icon-clock-outline'/> {'When is the next launch window?'}
        </SearchSuggestion>
        <SearchSuggestion>
            <i className='icon icon-clock-outline'/> {'Who is the launch director on call today?'}
        </SearchSuggestion>

        <SuggestionsHeader>{'Suggestions'}</SuggestionsHeader>
        <SearchSuggestion>
            <IconAI/> {'What problems have the most recent launches had?'}
        </SearchSuggestion>
        <SearchSuggestion>
            <IconAI/> {'What messages are pending my feedback?'}
        </SearchSuggestion>
        <SearchSuggestion>
            <IconAI/> {'What is currently my most active channel?'}
        </SearchSuggestion>
        <SearchSuggestion>
            <IconAI/> {'What is the current weather in Jacksonville?'}
        </SearchSuggestion>
    </SearchSuggestionsContainer>
);

export default SearchSuggestions;
