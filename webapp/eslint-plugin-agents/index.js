// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

const jsxA11y = require('eslint-plugin-jsx-a11y');

// jsx-a11y only recognizes lowercase DOM tags, so it skips <Header onClick>
// when Header is `styled.div`. These wrappers map each file's styled-components
// declarations to their DOM tag before delegating to the jsx-a11y rule.
const A11Y_RULES = [
    'alt-text',
    'anchor-has-content',
    'aria-activedescendant-has-tabindex',
    'aria-props',
    'aria-proptypes',
    'aria-role',
    'click-events-have-key-events',
    'control-has-associated-label',
    'iframe-has-title',
    'interactive-supports-focus',
    'no-noninteractive-element-interactions',
    'no-noninteractive-tabindex',
    'no-static-element-interactions',
    'role-has-required-aria-props',
    'role-supports-aria-props',
    'tabindex-no-positive',
];

function styledTag(node) {
    if (!node) {
        return null;
    }
    switch (node.type) {
    case 'TaggedTemplateExpression':
        return styledTag(node.tag);
    case 'CallExpression':
        if (node.callee.type === 'Identifier' && node.callee.name === 'styled') {
            const [arg] = node.arguments;
            if (arg && arg.type === 'Literal' && typeof arg.value === 'string') {
                return {tag: arg.value};
            }
            if (arg && arg.type === 'Identifier') {
                return {base: arg.name};
            }
            return null;
        }
        return styledTag(node.callee);
    case 'MemberExpression':
        if (node.object.type === 'Identifier' && node.object.name === 'styled' && node.property.type === 'Identifier') {
            return {tag: node.property.name};
        }
        return styledTag(node.object);
    case 'TSAsExpression':
    case 'TSInstantiationExpression':
        return styledTag(node.expression);
    default:
        return null;
    }
}

function styledComponentTags(ast) {
    const declared = new Map();
    const visit = (node) => {
        if (!node || typeof node.type !== 'string') {
            return;
        }
        if (node.type === 'VariableDeclarator' && node.id.type === 'Identifier') {
            const found = styledTag(node.init);
            if (found) {
                declared.set(node.id.name, found);
            }
        }
        for (const key of Object.keys(node)) {
            if (key === 'parent') {
                continue;
            }
            const child = node[key];
            if (Array.isArray(child)) {
                child.forEach(visit);
            } else if (child && typeof child.type === 'string') {
                visit(child);
            }
        }
    };
    visit(ast);

    const resolve = (name, seen) => {
        const found = declared.get(name);
        if (!found || seen.has(name)) {
            return null;
        }
        seen.add(name);
        return found.tag || resolve(found.base, seen);
    };

    const tags = {};
    for (const name of declared.keys()) {
        const tag = resolve(name, new Set());
        if (tag) {
            tags[name] = tag;
        }
    }
    return tags;
}

function withStyledComponents(rule) {
    return {
        meta: rule.meta,
        create(context) {
            const sourceCode = context.sourceCode || context.getSourceCode();
            const a11ySettings = context.settings['jsx-a11y'] || {};
            const settings = {
                ...context.settings,
                'jsx-a11y': {
                    ...a11ySettings,
                    components: {...styledComponentTags(sourceCode.ast), ...a11ySettings.components},
                },
            };
            return rule.create(Object.create(context, {settings: {value: settings}}));
        },
    };
}

function recommendedSeverity(name) {
    const entry = jsxA11y.configs.recommended.rules[`jsx-a11y/${name}`];
    return Array.isArray(entry) ? ['error', ...entry.slice(1)] : 'error';
}

module.exports = {
    rules: Object.fromEntries(A11Y_RULES.map((name) => [name, withStyledComponents(jsxA11y.rules[name])])),
    configs: {
        recommended: {
            plugins: ['agents'],
            settings: {'jsx-a11y': {polymorphicPropName: 'as'}},
            rules: Object.fromEntries(A11Y_RULES.map((name) => [`agents/${name}`, recommendedSeverity(name)])),
        },
    },
};
