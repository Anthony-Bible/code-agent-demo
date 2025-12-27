# Agent Skills

This directory contains agent skills following the [agentskills.io](https://agentskills.io) specification.

## About Skills

Skills are modular capabilities that extend the AI agent's functionality. Each skill is a self-contained directory with a `SKILL.md` file that defines its purpose and behavior.

## Available Skills

- `test-skill` - A test skill for validation and integration testing
- `code-review` - Performs code review with best practices

## Creating A New Skill

1. Create a new directory for your skill:
   ```bash
   mkdir skills/your-skill-name
   ```

2. Create a `SKILL.md` file:
   ```yaml
   ---
   name: your-skill-name
   description: A brief description of what this skill does
   license: MIT
   ---

   # Your Skill

   Detailed instructions, patterns, and examples.
   ```

3. (Optional) Add supporting directories:
   - `scripts/` - Executable scripts (Python, Bash, etc.)
   - `references/` - Documentation and reference materials
   - `assets/` - Templates, configs, and static resources

4. The skill will be automatically discovered at the next agent startup.

## Specification

See [agentskills.io/specification](https://agentskills.io/specification) for complete details on the skill format, validation rules, and best practices.

## Security Considerations

- Skills are executed using the existing tool system with all security controls
- Script execution requires user confirmation (via bash tool)
- Only skills from trusted sources should be used
- Review skill code and scripts before running in production environments
