# Project Recovery Init

`init.sh` is a recovery contract, not only a shell script.

## Two Init Layers

Installed projects have two different verification layers:

- `.agent-harness/scripts/init.sh` verifies the harness itself: required files, state validity, prompts, docs, tests, and harness examples.
- Root `./init.sh` is the project recovery entry point that future agents, humans, and CI run from a fresh checkout.

In hidden layout, the generated root `./init.sh` starts as a thin wrapper around `.agent-harness/scripts/init.sh`. That is only the pre-minspec state. It means the harness is installed and usable for planning; it does not mean project business behavior exists or has been smoke tested.

## Before Minspec

Before a project minspec exists, root `./init.sh` may only verify the harness. It must not claim that dependencies, services, endpoints, UI flows, jobs, or business logic are runnable.

`progress.md` should describe this state as a recoverable planning environment and name the next step: create or accept a minspec, then plan the runnable skeleton feature.

## After Minspec

Once a minspec is accepted, Planning Agent work must add a runnable-skeleton feature before product behavior features depend on runtime assumptions.

That feature must update root `./init.sh` so it:

- runs from a fresh checkout on a new machine with no chat context;
- is idempotent;
- installs or verifies declared dependencies;
- starts required local services or processes itself;
- does not assume a service is already running;
- performs at least one real smoke test against an endpoint or core function;
- emits clear `echo` logs for setup, start, test, and success or failure phases;
- exits non-zero when setup, startup, or smoke verification fails;
- avoids pseudocode, TODO placeholders, and manual steps.

The root project init may call `.agent-harness/scripts/init.sh` as one phase, but harness verification alone is not enough after the project has a runnable skeleton.

## Ownership

Project recovery belongs in project-owned source, setup, contract, and test paths. Default examples such as `examples/tiny-cli` and `examples/go-server` are harness demonstrations; they can guide implementation but cannot satisfy downstream project recovery.

## Evaluation

Evaluators must reject project-level completion when root `./init.sh` only proves the harness is installed but the accepted minspec requires a runnable project skeleton. Use `state_recovery_gap`, `capability_gap`, or `test_gap` as the primary failure domain depending on whether the missing piece is the recovery contract, a required runtime capability, or verification coverage.
