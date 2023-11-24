import styled from 'styled-components';

export const Button = styled.button`
    border-radius: 4px;
    padding: 8px 16px;
    display: flex;
    align-items: center;
    font-weight: 600;
    font-size: 12px;
    background-color: rgb(var(--center-channel-bg-rgb));
    border: 0;

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

	i {
		display: flex;
		font-size: 14px;
		margin-right: 2px;
	}
`;
