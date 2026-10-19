// Posts the eval report produced by `evalviewer report` on the pull request.
// The comment exists only while there is something actionable: it is created
// or updated when the report lists regressions, and an earlier comment is
// marked resolved once they are gone. Passing runs never add a comment.

const fs = require('fs');

// Matches the heading written by evalviewer, including comments from earlier
// versions of this workflow.
const MARKER = '## 🤖 LLM Evaluation Results';
const RESOLVED = '✅ No eval regressions';
const PROVIDERS = /<!-- eval-providers: ([^>]*) -->/;

// Providers the existing comment reports on that have no usable results in
// this run. Their failures are missing from the report, not fixed.
function providersWithoutResults(existingBody, summaryPath) {
    const match = existingBody.match(PROVIDERS);
    const listed = match ? match[1].split(',').filter(Boolean) : [];
    if (listed.length === 0) {
        return [];
    }
    const summary = fs.existsSync(summaryPath) ? JSON.parse(fs.readFileSync(summaryPath, 'utf8')) : {providers: []};
    const available = new Set(summary.providers.filter((p) => p.available).map((p) => p.name));
    return listed.filter((name) => !available.has(name));
}

// Keeps providers that stopped reporting in a new comment, so their earlier
// failures still block resolving it, and says that they were not rechecked.
function carryForward(body, missing) {
    const match = body.match(PROVIDERS);
    const current = match ? match[1].split(',').filter(Boolean) : [];
    const providers = [...new Set([...current, ...missing])].sort();
    const note = `No usable results for ${missing.join(', ')} in this run, so earlier failures there were not rechecked.\n\n`;
    const line = `<!-- eval-providers: ${providers.join(',')} -->\n`;
    if (match) {
        return body.replace(`${match[0]}\n`, line + note);
    }
    return body.replace(`${MARKER}\n\n`, `${MARKER}\n\n${line}${note}`);
}

async function postEvalComment(github, context, commentPath, summaryPath) {
    const body = fs.existsSync(commentPath) ? fs.readFileSync(commentPath, 'utf8').trim() : '';
    const repo = {owner: context.repo.owner, repo: context.repo.repo};

    const comments = await github.paginate(github.rest.issues.listComments, {
        ...repo,
        issue_number: context.issue.number,
        per_page: 100,
    });
    const existing = comments.find((comment) => comment.user.type === 'Bot' && comment.body.includes(MARKER));
    const unresolved = existing && !existing.body.includes(RESOLVED);
    const missing = unresolved ? providersWithoutResults(existing.body, summaryPath) : [];

    if (body) {
        const newBody = missing.length > 0 ? carryForward(body, missing) : body;
        if (existing) {
            await github.rest.issues.updateComment({...repo, comment_id: existing.id, body: newBody});
            console.log(`Updated eval comment ${existing.id}`);
        } else {
            const {data} = await github.rest.issues.createComment({...repo, issue_number: context.issue.number, body: newBody});
            console.log(`Created eval comment ${data.id}`);
        }
        return;
    }

    if (unresolved) {
        if (missing.length > 0) {
            console.log(`Keeping eval comment ${existing.id}: no usable results for ${missing.join(', ')}`);
            return;
        }
        const sha = context.payload.pull_request.head.sha.slice(0, 7);
        await github.rest.issues.updateComment({
            ...repo,
            comment_id: existing.id,
            body: `${MARKER}\n\n${RESOLVED} as of ${sha}.\n`,
        });
        console.log(`Marked eval comment ${existing.id} resolved`);
        return;
    }

    console.log('Nothing actionable; no eval comment posted.');
}

module.exports = {postEvalComment, providersWithoutResults, carryForward};
