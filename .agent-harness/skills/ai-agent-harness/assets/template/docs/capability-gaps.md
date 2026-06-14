# Capability Gaps

The harness should make missing capabilities visible and fixable. A capability gap exists when a tool, permission, generator, dependency, service, credential, runtime setting, CI resource, or verification fixture is required to implement or verify a feature durably but is missing, unclear, or unavailable.

Capability gaps are not ordinary implementation details. They affect whether future agents and CI can reproduce the work.

## Required Response

When a capability gap appears:

1. Verify the gap with real evidence such as official documentation, tool help, a minimal command, captured logs, or CI output.
2. Decide whether the selected feature should provide the capability now.
3. If yes, add a durable project capability: setup documentation, dependency declaration, script, adapter, checked-in fixture, CI configuration, or regression test.
4. If no, record the gap in `runs/`, `progress.md`, or `last_error`, and mark the feature blocked or append a follow-up feature.
5. Re-run the relevant verification after the durable capability is in place.

## Workaround Rules

Do not treat these as durable completion:

- Hand-writing generated bindings or generated artifacts only because the generator is missing.
- Skipping tests, narrowing acceptance criteria, or relying on assistant prose because a tool is unavailable.
- Setting local-only environment variables that are not captured in scripts, Make targets, CI, or setup docs.
- Depending on credentials, services, network access, or permissions that future agents cannot discover from repository files.

Temporary workarounds are allowed only to inspect, debug, or unblock local evidence gathering. They must be labeled temporary and recorded. A feature can pass only when the capability is provided, explicitly scoped out, or converted into a tracked follow-up.

## Examples

If `protoc` or another code generator is required, do not hand-write equivalent generated bindings merely to avoid installing or configuring the generator. Add the generator workflow, commit generated output with its source and regeneration command, choose a supported library path, or block the feature with a follow-up capability feature.

If `go test` fails because the default Go build cache is not writable, setting `GOCACHE` under a temporary directory is acceptable for local verification only when the project also records that behavior in `init.sh`, a Make target, CI configuration, or setup docs.

If a third-party API, deployment CLI, or model output schema is needed, capture a primary-source shape or real-shaped fixture before parsing or trusting it.

If orchestrator-first work cannot run because `agent-provider.json` is missing, the configured provider command is unavailable, or multiple provider CLIs exist without an explicit selection, treat that as an agent provider capability gap. Add durable provider configuration, adapter documentation, or a follow-up feature instead of silently falling back to manual completion.

If a configured provider runtime check reports `PROVIDER_RUNTIME_PERMISSION_REQUIRED`, treat it as an agent provider permission capability gap. The orchestrator must stop before mutating feature state, and the outer agent or user must explicitly approve escalated provider runtime execution before retrying.

## Evaluation

Evaluators should fail a feature when a required capability is bypassed instead of made durable. Use `capability_gap` as the primary failure domain when the main problem is a missing or implicit capability.
