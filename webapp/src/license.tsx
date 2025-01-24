// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import {GlobalState} from '@mattermost/types/store';

const e10 = 'E10';
const e20 = 'E20';
const professional = 'professional';
const enterprise = 'enterprise';

// isValidSkuShortName returns whether the SKU short name is one of the known strings;
// namely: E10 or professional, or E20 or enterprise
const isValidSkuShortName = (license: Record<string, string>) => {
    switch (license?.SkuShortName) {
    case e10:
    case e20:
    case professional:
    case enterprise:
        return true;
    default:
        return false;
    }
};

const checkE20Licensed = (license: Record<string, string>) => {
    if (license?.SkuShortName === e20 || license?.SkuShortName === enterprise) {
        return true;
    }

    if (!isValidSkuShortName(license)) {
        // As a fallback for licenses whose SKU short name is unknown, make a best effort to try
        // and use the presence of a known E20/Enterprise feature as a check to determine licensing.
        if (license?.MessageExport === 'true') {
            return true;
        }
    }

    return false;
};

const checkE10Licensed = (license: Record<string, string>) => {
    if (license?.SkuShortName === e10 || license?.SkuShortName === professional ||
        license?.SkuShortName === e20 || license?.SkuShortName === enterprise) {
        return true;
    }

    if (!isValidSkuShortName(license)) {
        // As a fallback for licenses whose SKU short name is unknown, make a best effort to try
        // and use the presence of a known E10/Professional feature as a check to determine licensing.
        if (license?.LDAP === 'true') {
            return true;
        }
    }

    return false;
};

const isConfiguredForDevelopment = (state: GlobalState): boolean => {
    const config = state.entities.general.config;

    return config.EnableTesting === 'true' && config.EnableDeveloper === 'true';
};

// isE20LicensedOrDevelopment returns true when the server is licensed with a legacy Mattermost
// Enterprise E20 License or a Mattermost Enterprise License, or has `EnableDeveloper` and
// `EnableTesting` configuration settings enabled, signaling a non-production, developer mode.
export const isE20LicensedOrDevelopment = (state: GlobalState): boolean => {
    const license = state.entities.general.license;

    return checkE20Licensed(license) || isConfiguredForDevelopment(state);
};

// isE10LicensedOrDevelopment returns true when the server is at least licensed with a legacy Mattermost
// Enterprise E10 License or a Mattermost Professional License, or has `EnableDeveloper` and
// `EnableTesting` configuration settings enabled, signaling a non-production, developer mode.
export const isE10LicensedOrDevelopment = (state: GlobalState): boolean => {
    const license = state.entities.general.license;

    return checkE10Licensed(license) || isConfiguredForDevelopment(state);
};

export function useIsMultiLLMLicensed() {
    return useSelector(isE20LicensedOrDevelopment);
}

export function useIsBasicsLicensed() {
    return useSelector(isE20LicensedOrDevelopment);
}
