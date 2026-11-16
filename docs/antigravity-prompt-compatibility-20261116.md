# Antigravity system prompt compatibility

## Approved behavior

Only API keys directly bound to an `antigravity` platform group receive the new
rules. The current deployment has one such group: ID 8. Keys in Gemini or
composite groups do not receive these rules even when selecting an Antigravity
account. Existing legacy neutralization outside this scope remains unchanged.

Rewrite the supplied Codex, Claude Code and Claude Agent SDK identity sentences
to `You are a helpful coding assistant.`. Support ASCII and curly apostrophes.
Remove complete, syntactically matching `x-anthropic-billing-header:` prompt
lines. Preserve malformed lines and ordinary mentions. OpenCode replacement
was explicitly excluded by the user.

The marked Cursor request was sampled on 2026-11-16 at 14:48:38 UTC, before
committing or deploying prompt changes. It used Gemini native streaming and
contained these system identity lines:

- `You are an AI coding assistant, powered by gemini-4-hydrogen-xhigh.`
- `You are a coding agent in the Cursor IDE that helps the USER with software engineering tasks.`

Rewrite those sentence forms to the same coding assistant identity. The first
rule permits other Gemini model IDs in that exact identity sentence; it does
not globally replace model IDs or vendor mentions. Only identity-line snippets
were recorded, and the temporary proxy route and process were removed after
sampling. No sampling code is included in the application release.

## Integration

Apply the replacements at the Antigravity service entrypoints, before existing
protocol conversion and identity neutralization. Handle Claude `system`, Gemini
`systemInstruction`/`system_instruction`, Responses `instructions` plus
system/developer input messages, and Chat Completions system/developer messages.
Modify only actual text blocks; leave tool schemas, tool calls, tool outputs,
user/assistant content, paths, model names and metadata untouched. Use parsed
JSON field selection and targeted JSON string updates, not whole-body regexes.
Keep original ingress security auditing and the mandatory upstream Antigravity
identity patch intact. Invalid requests retain the existing error behavior.

## Verification and release

Test each rule, multiline/CRLF inputs, paths, idempotency, protected fields and
group isolation. Exercise all four real forwarding entrypoints with a mock
upstream and rerun existing identity and Gemini routing regressions. Compile a
new backend on top of the deployed source and preserve current runtime resources.
Commit only relevant source, tests and this design; do not commit credentials or
local deployment overrides. Include the previously deployed Gemini endpoint fix
in a separate commit so the repository represents the deployed baseline.

Back up the current override and touched source, deploy only the application
service, check health and native endpoints, and retain the previous image for
rollback. Do not enable persistent prompt-body logging. User-requested Cursor
sampling has completed; deploy all identity rules together in this release.
