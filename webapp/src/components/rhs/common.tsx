import styled from 'styled-components';

export const Button = styled.button`
    border-radius: 4px;
    padding: 8px 16px;
    display: flex;
    align-items: center;
    font-weight: 600;
    font-size: 12px;
    background-color: rgb(var(--center-channel-bg-rgb));
    color: rgba(var(--center-channel-color), 0.6);
    width: 172px;
    border: 0;
    margin: 0 8px 8px 0;

    &:hover {
        background-color: rgba(var(--button-bg-rgb), 0.08);
        color: rgb(var(--link-color-rgb));
        svg {
            fill: rgb(var(--link-color-rgb))
        }
    }

    svg {
        fill: rgb(var(--center-channel-color));
        margin-right: 6px;
    }
`;
