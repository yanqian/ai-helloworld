# Example Boundaries

The `examples/` directory exists to prove and explain the harness. It is not the default product surface for new project requirements.

The default examples are small fixtures:

- `examples/tiny-cli/` proves dependency-free Python verification.
- `examples/go-server/` proves a dependency-free Go service can be verified by the harness.

## Rules

- Modify a default example only when the selected feature explicitly says to change that example or the harness demonstration itself.
- Do not implement a project-level feature by repurposing `examples/tiny-cli/`, `examples/go-server/`, or another default example.
- Put product code in project-owned paths such as `src/`, `cmd/`, `internal/`, `pkg/`, `app/`, `lib/`, `services/`, `contracts/`, or another path named by the project.
- Put product verification in project-owned tests or contract fixtures, then wire those checks into `./init.sh`.
- During fresh project setup, it is acceptable to remove or replace default examples, but that should be explicit setup work rather than an implementation shortcut.

## Example

If a user asks for project-level protobuf or gRPC behavior, do not turn `examples/go-server/` into that product. Create or identify the project's Go module, service, proto or contract path, tests, and verification command. The Go server example remains only a reference unless the feature explicitly says to change the example.

## Evaluation

Evaluators should fail a project-level feature when its acceptance criteria are met only by changing default examples. Use `example_scope_gap` as the primary failure domain when the main problem is that product work was implemented in the example surface.
