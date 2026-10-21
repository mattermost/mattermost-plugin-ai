// testcontainers' exec resolves even when the command exits non-zero, so an
// unchecked call lets a failed setup step pass silently.

function isContainerExec(call) {
    const {callee} = call;
    if (callee.type !== 'MemberExpression' || callee.computed || callee.property.name !== 'exec') {
        return false;
    }
    const receiver = callee.object;
    const name = receiver.type === 'MemberExpression' ? receiver.property.name : receiver.name;
    return typeof name === 'string' && /container$/i.test(name);
}

function readsExitCode(call) {
    let node = call.parent;
    if (node.type === 'AwaitExpression') {
        node = node.parent;
    }
    if (node.type === 'MemberExpression' && node.property.name === 'exitCode') {
        return true;
    }
    if (node.type === 'VariableDeclarator' && node.id.type === 'ObjectPattern') {
        return node.id.properties.some((p) => p.type === 'Property' && p.key.name === 'exitCode');
    }
    return false;
}

export default {
    meta: {
        type: 'problem',
        docs: {description: 'Check the exit code of container.exec'},
        messages: {
            unchecked: 'container.exec resolves on non-zero exit; destructure and check exitCode (or use mattermost.mmctl).',
        },
        schema: [],
    },
    create(context) {
        return {
            CallExpression(call) {
                if (isContainerExec(call) && !readsExitCode(call)) {
                    context.report({node: call, messageId: 'unchecked'});
                }
            },
        };
    },
};
