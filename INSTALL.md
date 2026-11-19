# Installation

Install with [skills](https://skills.sh/) CLI (universal, works with any [Agent Skills](https://agentskills.io)-compatible tool):

```bash
npx skills add https://github.com/dash0hq/agent-skills --all
# or a single skill:
npx skills add https://github.com/dash0hq/agent-skills --skill otel-semantic-conventions
```

## Packaged archives

Each [GitHub release](https://github.com/dash0hq/agent-skills/releases) attaches these archives:

| Archive | Contents | Use it for |
|---|---|---|
| `dash0-agent-skills.zip` | Every skill folder, side by side | Installing all skills by hand into a tool's skills directory (preferred) |
| `<skill>.skill` | 1 skill folder with `SKILL.md` inside it | Uploading to the Claude apps |
| `<skill>.zip` | Same content as `<skill>.skill` | Installing a single skill by hand |

The skills link to each other's rules, so you should install or upload all of them to keep those links working.

To add the skills in the Claude apps, open Customize > Skills, select + > Create skill > Upload a skill, and upload each `.skill` file in turn.
The Claude apps accept 1 skill per upload, so they refuse `dash0-agent-skills.zip`.

To install by hand, unpack the bundle into the tool's skills directory.
Remove the previous copies first, so rule files that a newer release deleted do not linger:

```bash
curl -fsSLO https://github.com/dash0hq/agent-skills/releases/latest/download/dash0-agent-skills.zip
rm -rf "${HOME}"/.claude/skills/otel-*
unzip -q dash0-agent-skills.zip -d "${HOME}/.claude/skills/"
```

## Tessl

```bash
tessl install dash0/agent-skills
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

Install the plugin, or copy the skills into the cross-client discovery directory:

```bash
/plugin install https://github.com/dash0hq/agent-skills
# or
git clone https://github.com/dash0hq/agent-skills.git ~/.copilot/skills/dash0-agent-skills
```

The `/plugin install` form reads `plugin.json` at the repository root, the portable [Agent Plugins](https://agent-plugins.org/) 1.0.0 manifest that Copilot CLI and marketplaces in that format consume; Copilot discovers the skills from `skills/` next to it.
Its version follows the release tag.
The `git clone` form skips the manifest: Copilot auto-discovers skills from `.copilot/skills/`.

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
