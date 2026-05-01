# Installation

Install with [skills](https://skills.sh/) CLI (universal, works with any [Agent Skills](https://agentskills.io)-compatible tool):

```bash
npx skills add https://github.com/dash0hq/agent-skills --all
# or a single skill:
npx skills add https://github.com/dash0hq/agent-skills --skill otel-semantic-conventions
```

## Antigravity

Clone and symlink into the cross-client discovery path:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.antigravity/skills/dash0-agent-skills
```

Update with `git -C ~/.antigravity/skills/dash0-agent-skills pull`.

## Claude Code

```bash
/plugin marketplace add dash0hq/claude-marketplace
/plugin install dash0-agent-skills@dash0
```

## Codex (OpenAI)

Clone into the cross-client discovery path:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.agents/skills/dash0-agent-skills
```

Codex auto-discovers skills from `~/.agents/skills/` and `.agents/skills/`.
Update with `git -C ~/.agents/skills/dash0-agent-skills pull`.

## Copilot

Copy skills into the cross-client discovery directory:

```bash
/plugin install https://github.com/dash0hq/agent-skills
# or
git clone https://github.com/dash0hq/agent-skills.git ~/.copilot/skills/dash0-agent-skills
```

Copilot auto-discovers skills from `.copilot/skills/`.

## Cursor

Copy skills into the cross-client discovery directory:

```bash
git clone https://github.com/dash0hq/agent-skills.git  ~/.cursor/skills/dash0-agent-skills
```

Cursor auto-discovers skills from `.agents/skills/` and `.cursor/skills/`.

## Gemini CLI

```bash
gemini extensions install https://github.com/dash0hq/agent-skills
```

Update with `gemini extensions update dash0-agent-skills`.

## Openclaw

Copy skills into the cross-client discovery directory:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.openclaw/skills/dash0-agent-skills
# or in workspace:
git clone https://github.com/dash0hq/agent-skills.git ~/.openclaw/workspace/skills/dash0-agent-skills
```

## OpenCode

Copy skills into the cross-client discovery directory:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.agents/skills/dash0-agent-skills
```

OpenCode auto-discovers skills from `.agents/skills/`, `.opencode/skills/`, and `.claude/skills/`.

## Tessl

```bash
tessl install dash0/agent-skills
```
