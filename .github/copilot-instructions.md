GOAL

Enforce safe and testable fixes, maintain independence between "internal" and "gui", require Svelte 5 documentation verification (if needed), and validate only via allowed commands.

RULES

- Fixes MUST include reproducible debugging steps, temporary logging, exact command outputs, and tests proving correctness. Follow the full DEBUGGING PROCESS.
- "internal" pkg MUST NEVER import the "gui" pkg. The "gui" pkg MAY import "internal".
- PROHIBITED commands: "go run ./cmd/bot/main.go", "wails dev", "bun run dev", "mise run dev-cli", "mise run dev-gui". REASON: These commands block indefinitely waiting for a QR code scan, which will halt and fail your execution environment.
- Frontend code MUST use Svelte 5 runes ($state, $derived). Do NOT generate Svelte 3 or 4 code.

SVELTE DOCUMENTATION REQUIREMENTS

- You MUST consult the official Svelte 5 docs for ALL frontend changes. Your training data is outdated.
- Primary docs URL: https://svelte.dev/docs/llms
- For EACH frontend change, you MUST include the exact documentation pages consulted (URLs) and a one-line rationale linking the change to the doc section used.

TOOLING & VALIDATION COMMANDS

You MUST use mise for all root-level tasks, defined in mise.toml.

- "mise run test": Run all Go tests.
- "mise run build-cli": Build the command-line application.
- "mise run build-gui": Build the GUI application.
- "mise run fmt": Format all Go and frontend code.
- For frontend-specific tasks, "cd" into "gui/frontend" first:
  - "bun install"
  - "bun run lint"
  - "bun run check"

DEBUGGING PROCESS

Your process to fix bugs MUST follow these exact steps:

1. REPRODUCE. Start by writing or documenting a minimal reproduction (unit test or integration test) that demonstrates the bug.
2. ASSERT FAILURE. Add a failing test that reproduces the bug and run "mise run test" to capture failing output.
3. ADD LOGGING. Add temporary structured logging using internal/logger slog
4. CAPTURE LOGS. Run the failing test or the relevant test subset and capture logs and test output. Include raw output in submission.
5. ANALIZE. Analyze logs to trace variable values and program flow. Document what was inspected and why.
6. HYPOTHESIZE. State the minimal hypothesis for root cause based on logs and tests.
7. IMPLEMENT. Make the smallest code change that addresses the hypothesis. Prefer changes in internal packages only and maintain package boundaries.
8. UPDATE TESTS. Update the failing test or add new tests to assert the correct behavior.
9. ASSERT PASS. Run "mise run test". Capture the complete passing output.
10. BUILD. Run "mise run build-cli" and "mise run build-gui". Capture outputs.
11. CLEANUP. Remove temporary logging or mark it clearly. Re-run tests and builds to confirm no regressions.
12. FORMAT. Run "mise run fmt". Capture output.
13. DOCUMENT. Assemble the final submission according to the SUBMISSION CONTENTS section. If code is complex, add a comment explaining the change.

SUBMISSION CONTENTS

Your PR description must contain the following sections with raw outputs in code blocks:

1. Summary: one-line bug id and one-line fix summary.
2. Failing test output: raw captured output.
3. Added/updated tests: file paths and diffs.
4. Passing test output: raw captured output.
5. If applicable. Svelte docs references: list of URLs consulted and one-line rationale per URL.

ACCEPTANCE CRITERIA

- "mise run build-cli" and "mise run build-gui" succeed.
- "mise run test" passes all tests.
- "cd gui/frontend && bun run lint" passes.
- "cd gui/frontend && bun run check" passes.
- Submission includes all items listed in SUBMISSION CONTENTS.
- No "internal" => "gui" dependency is introduced.

REPOSITORY STRUCTURE

- cmd/bot/: CLI entry point.
- gui/: Wails GUI application.
  - gui/main.go and gui/app.go: Go code that binds the backend logic to the frontend.
  - gui/frontend/: The Svelte 5 + TypeScript frontend source code.
- internal/: All core backend logic, which should not have any dependency on the gui pkg.
  - internal/config: Configuration loading (.env).
  - internal/domain: Core data structures (e.g., Flow, UserState).
  - internal/service: Business logic, FSM, actions, BotController.
  - internal/repository: Database interaction layer (SQLite).
  - internal/platform: Adapters for external services (WhatsApp client, database driver).
  - internal/logger: Custom slog setup.
