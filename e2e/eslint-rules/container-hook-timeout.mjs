// A beforeAll hook shares the 60 s per-test timeout unless it sets its own,
// and container startup alone can exceed that. Container starters follow the
// `Run<Name>` naming convention (plus start*Stack harnesses).

const STARTER = /^(Run[A-Z]\w*|start\w*Stack)$/;

function isHook(call) {
    const {callee} = call;
    return callee.type === 'MemberExpression' && !callee.computed && callee.property.name === 'beforeAll';
}

function isSetTimeout(statement) {
    if (!statement || statement.type !== 'ExpressionStatement' || statement.expression.type !== 'CallExpression') {
        return false;
    }
    const {callee} = statement.expression;
    return callee.type === 'MemberExpression' && callee.property.name === 'setTimeout' && callee.object.type === 'Identifier' && callee.object.name !== 'window';
}

function findStarter(node, visitorKeys) {
    if (!node || typeof node.type !== 'string') {
        return null;
    }
    if (node.type === 'CallExpression' && node.callee.type === 'Identifier' && STARTER.test(node.callee.name)) {
        return node;
    }
    for (const key of visitorKeys[node.type] ?? []) {
        const child = node[key];
        for (const item of Array.isArray(child) ? child : [child]) {
            const found = findStarter(item, visitorKeys);
            if (found) {
                return found;
            }
        }
    }
    return null;
}

export default {
    meta: {
        type: 'problem',
        docs: {description: 'Container-starting beforeAll hooks set their own timeout first'},
        messages: {
            missing: 'This beforeAll starts containers via {{starter}}(); call test.setTimeout(...) as its first statement.',
        },
        schema: [],
    },
    create(context) {
        const {visitorKeys} = context.sourceCode;
        return {
            CallExpression(call) {
                if (!isHook(call)) {
                    return;
                }
                const callback = call.arguments.find((arg) => arg.type === 'ArrowFunctionExpression' || arg.type === 'FunctionExpression');
                if (!callback || callback.body.type !== 'BlockStatement') {
                    return;
                }
                const starter = findStarter(callback.body, visitorKeys);
                if (!starter || isSetTimeout(callback.body.body[0])) {
                    return;
                }
                context.report({node: call, messageId: 'missing', data: {starter: starter.callee.name}});
            },
        };
    },
};
