import {useState} from 'react';

let idCounter = 0;

// uniqueId generates a unique id with an optional prefix.
export const uniqueId = (prefix ?: string) => prefix + String(++idCounter);

// useUniqueId exports a React hook simplifying the use of uniqueId.
//
// Note that changes to the prefix will not effect a change to the unique identifier.
export const useUniqueId = (prefix ?: string) => {
    const [id] = useState(() => uniqueId(prefix));

    return id;
};
