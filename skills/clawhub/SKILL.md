---
name: clawhub
description: Use the ClawHub CLI to search, install, and update agent skills from clawhub.ai. Use when you need to find new skills (e.g., web scraping, data analysis, trending skills), install capabilities on the fly, or sync installed skills to latest version.
homepage: https://clawhub.ai
metadata: {"lingguard":{"emoji":"🦞"}}
---

# ClawHub

Public skill registry for AI agents. Search by natural language (vector search).

## When to use

Use this skill when the user asks about:
- Finding or searching for skills
- Popular, trending, or hot skills
- Installing new skills
- Updating installed skills

## Search

```bash
npx --yes clawhub@latest search "web scraping" --limit 5
```

## Install

**IMPORTANT: Use --workdir with absolute path, do NOT use cd command!**

```bash
npx --yes clawhub@latest install <slug> --workdir "$HOME/.lingguard/workspace"
```

Replace `<slug>` with the skill name from search results.

**If security warning appears:** Ask user for confirmation, then use `--force`:
```bash
npx --yes clawhub@latest install <slug> --workdir "$HOME/.lingguard/workspace" --force
```

## Update

```bash
npx --yes clawhub@latest update --all --workdir "$HOME/.lingguard/workspace"
```

## List installed

```bash
npx --yes clawhub@latest list --workdir "$HOME/.lingguard/workspace"
```

## Notes

- Requires Node.js (`npx` comes with it)
- After install, remind user to start a new session to load the skill
- Skills from ClawHub override builtin skills with the same name
