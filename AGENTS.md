# AGENTS.md

This is a Terraform provider for Estuary captures and materializations. It does its work by running the flowctl CLI, not by calling an API. Each connector resource is built from its spec in `internal/catalog`.

Start with the Development Guide in `README.md`. It covers the layout, adding a connector, testing, docs and submitting changes. This file adds the parts the guide leaves out.

## Before committing

- Run the steps in `.github/workflows/tests.yaml`, including `go test -race`. Then run `go generate ./...` and commit the regenerated `docs/` with your change. CI doesn't check that `docs/` is current, and the pre-commit hook only runs where `core.hooksPath` is set.
- An attribute added in `internal/resources/builder/codec.go` appears on every resource. It also needs a row in the attribute table of every connector README. `git grep -l catalog_name -- 'internal/resources/*/README.md'` lists those READMEs.

## Customer data

Every file git tracks uses invented names under a made-up tenant such as `acmeCo/`. That covers code, tests, fixtures, examples, docs and commit messages. When you reproduce a customer's setup, keep the shape (name length, nesting depth), drop their identifiers, and keep any real configuration in an untracked folder.

## Comments

- Doc comments are one or two lines. They start with the identifier and say what the code does. Inline comments say why: the reason, the context, or the detail that isn't obvious.
- Describe the code as it is now. Leave out what it used to do and the alternatives that weren't chosen.
- When you edit a comment, check each claim against the code it names, and remove as much as you add.

## Test doubles

- `testBackend` in `internal/resources/builder/resource_test.go` implements `catalog.Backend` in memory. A new `Backend` method needs an implementation there too.
- `recordingFlowctl` in `internal/flowctl/commands_test.go` records each flowctl command line, so tests can assert on the commands a change runs.

## Live Estuary

`FLOW_AUTH_TOKEN` in your environment authenticates flowctl and the provider. Pass it to commands as it is, and keep its value out of output. `flowctl raw graphql` queries the control-plane API, and `flowctl raw graphql-schema` prints its schema. Get the user's go-ahead before publishing or deleting live catalog entries.

## Commits

Name branches and commits as the Submitting Changes section of `README.md` says. Commit messages and PR descriptions hold the author's own text only, with no attribution lines.

## Agent skills

### Issue tracker

Issues live in this repo's GitHub Issues, used through `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

The five default triage labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` and `docs/adr/` at the root, created when first needed. See `docs/agents/domain.md`.
