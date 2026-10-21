import containerExecExitCode from './container-exec-exit-code.mjs';
import containerHookTimeout from './container-hook-timeout.mjs';
import scopedToolLabel from './scoped-tool-label.mjs';

export default {
    rules: {
        'container-exec-exit-code': containerExecExitCode,
        'container-hook-timeout': containerHookTimeout,
        'scoped-tool-label': scopedToolLabel,
    },
};
