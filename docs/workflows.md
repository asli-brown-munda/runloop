# Workflow Authoring

Runloop workflows are YAML files stored in the configured workflow directory. `runloop init` creates the directory and a sample `manual-hello.yaml`.

Default location:

```text
~/.config/runloop/workflows/
```

Isolated development location:

```text
$RUNLOOP_DEV_HOME/.config/runloop/workflows/
```

The daemon watches `.yaml` and `.yml` workflow files while it is running. Valid edits create a new stored workflow version. Invalid edits are logged and the previous valid version remains available.

## Minimal Workflow

```yaml
id: manual-hello
name: Manual Hello
enabled: true

triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    policy: once_per_item

steps:
  - id: echo
    type: transform
    input:
      message: "{{ inbox.normalized.message }}"
    output:
      result: "Hello from Runloop: {{ input.message }}"

sinks:
  - type: markdown
    path: report.md
```

## Top-Level Fields

- `id`: Stable workflow ID. Required.
- `name`: Human-readable workflow name. Required.
- `enabled`: Whether triggers can dispatch this workflow.
- `permissions.shell`: Required for `shell`, `claude`, and `git_checkout` steps.
- `triggers`: One or more triggers. Required.
- `steps`: One or more sequential steps. Required.
- `sinks`: Optional final outputs written after all steps succeed.

## Triggers

Current trigger type:

```yaml
triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    policy: once_per_item
```

Fields:

- `type`: Use `inbox`.
- `source`: Source ID to match, such as `manual`, `notes`, `heartbeat`, or `github-assigned-prs`.
- `entityType`: Entity type emitted by that source.
- `policy`: Dispatch policy. Defaults to `once_per_item` when omitted.

Policies:

- `once_per_item`: Run at most once for each inbox item and workflow.
- `once_per_version`: Run once for each changed inbox item version and workflow version.
- `manual_only`: Load the workflow without automatic dispatch.

Common source/entity pairs:

- Manual inbox: `source: manual`, `entityType: manual_item`
- Filesystem source default: `entityType: file_item`
- Schedule source default: `entityType: schedule_tick`
- GitHub PR source with unresolved review threads: `entityType: github_pr_unresolved_review_threads`
- GitHub PR source with no unresolved review threads: `entityType: github_pr_review_clean`

## Template Values

String values in `step.input`, `step.output`, `step.prompt`, and `step.workdir` can use simple `{{ path.to.value }}` templates.

Available context:

- `inbox.id`
- `inbox.source`
- `inbox.externalID`
- `inbox.entityType`
- `inbox.title`
- `inbox.raw`
- `inbox.normalized`
- `input`: The current step's rendered input.
- `runloop.workspace`: Per-run scratch directory.
- `steps.<step-id>`: The most recently completed step result, available to the next step.

Example:

```yaml
workdir: "{{ steps.checkout.path }}"
```

Templates are strict. A missing value fails the step with a clear missing template value error.

## Step Types

### transform

Use `transform` to reshape data without shelling out.

```yaml
steps:
  - id: summarize
    type: transform
    input:
      message: "{{ inbox.normalized.message }}"
    output:
      result: "Message: {{ input.message }}"
```

The `output` map becomes the step result.

### shell

Use `shell` for local commands. The workflow must opt in to shell permission.

```yaml
permissions:
  shell: true

steps:
  - id: command
    type: shell
    timeout: 30s
    command: "printf '%s\n' \"$MESSAGE\""
    env:
      MESSAGE: hello
```

Behavior:

- Runs through `sh -c`.
- Defaults to a 30 second timeout.
- Uses `command` and `env` as literal values; they are not template-rendered in the current MVP.
- Does not inherit the daemon's full environment.
- Receives only minimal defaults plus explicit `env` entries.
- Writes `stdout.log` and `stderr.log` step artifacts.

Environment entries can be:

```yaml
env:
  LITERAL_VALUE: hello
  DIRECT_SECRET:
    secret: profile-token
  LEGACY_PROFILE_VALUE:
    from: demo.PROFILE_TOKEN
```

### wait

Use `wait` for a simple in-process delay.

```yaml
steps:
  - id: pause
    type: wait
    duration: 5s
```

Durable wait/resume across daemon restarts is not implemented in the MVP.

### git_checkout

Use `git_checkout` to fetch a pull request head into the per-run workspace. The workflow must opt in to shell permission.

```yaml
permissions:
  shell: true

steps:
  - id: checkout
    type: git_checkout
    timeout: 2m
    input:
      repoURL: "{{ inbox.normalized.repository.cloneURL }}"
      pullNumber: "{{ inbox.normalized.number }}"
      pullRef: "{{ inbox.normalized.pullRef }}"
      headSHA: "{{ inbox.normalized.headSHA }}"
```

Required input:

- `repoURL`
- Either `pullRef` or `pullNumber`

Optional input:

- `headSHA`: Verifies the checked-out commit.
- `destination`: Relative path under `{{ runloop.workspace }}`. Defaults to `repo`.

Result values:

- `path`: Checkout directory.
- `headSHA`: Checked-out commit SHA.

### claude

Use `claude` to run the local Claude CLI. The workflow must opt in to shell permission.

```yaml
permissions:
  shell: true

steps:
  - id: agent
    type: claude
    timeout: 30m
    auth: auto
    connection: claude.default
    workdir: "{{ steps.checkout.path }}"
    permissionMode: acceptEdits
    prompt: |
      Address this inbox item:

      {{ inbox.normalized.reviewPromptMarkdown }}
```

Fields:

- `auth`: `auto`, `login`, or `apiKey`. Defaults to `auto`.
- `connection`: Preferred API-key connection, usually `claude.default`.
- `permissionMode`: Passed to Claude CLI as `--permission-mode`. Defaults to `plan`.
- `model`: Optional Claude model name passed as `--model`.
- `args`: Optional extra Claude CLI arguments.
- `prompt`: Required prompt body.

Credential behavior:

- `auth: login` relies on Claude CLI login state under the daemon's `HOME`.
- `auth: apiKey` requires an API-key source through `connection: claude.default` or legacy `profiles.claude`.
- `auth: auto` uses a configured connection first, then legacy `profiles.claude`, then Claude CLI login state.

Run `workflows show` to see readiness diagnostics:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop workflows show github-pr-claude
```

## Sinks

Sinks write final output after all steps succeed. Sink paths are relative to the run's sink artifact directory.

### markdown

```yaml
sinks:
  - type: markdown
    path: report.md
    body: |
      # Result

      {{ result }}
```

If `body` is omitted, Runloop writes a simple markdown report from the final step result.

### json

```yaml
sinks:
  - type: json
```

The JSON sink writes the final step result to `report.json`.

### file

```yaml
sinks:
  - type: file
    path: result.txt
    body: |
      Result: {{ .result }}
```

The file sink uses Go template syntax over the final step result.

## Validation Rules

Workflow loading rejects:

- Missing workflow `id` or `name`.
- Workflows without triggers or steps.
- Steps without IDs.
- Duplicate step IDs.
- Unsupported step types.
- Unsupported sink types.
- Malformed step environment entries.
- Unknown trigger fields, with YAML line context.

Use:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop workflows show <workflow-id>
```

to inspect the stored workflow version, persisted YAML, dispatch history, and readiness diagnostics.
