# Specification: Implement Automated Root Cause Analysis for Prometheus Alerts

## Overview
This track aims to enhance the existing alert investigation system by adding an automated Root Cause Analysis (RCA) engine. Currently, the agent can investigate alerts and gather findings, but it lacks a structured way to correlate these findings into a definitive root cause with suggested remedies.

## Objectives
- Define a structured data model for RCA findings, causes, and remedies.
- Implement logic to correlate disparate investigation findings into a cohesive RCA report.
- Leverage Claude's reasoning (Extended Thinking) to infer root causes from logs, metrics, and code traces.
- Provide actionable remediation suggestions at the end of every investigation.

## Core Components
1. **RCA Data Model:** New entities in `internal/domain/entity/` to track causes and remedies.
2. **Correlation Engine:** Logic in `internal/application/usecase/` to process investigation actions and findings.
3. **LLM Prompting:** Enhanced prompts in `internal/application/usecase/investigation_prompt_builder.go` to focus on RCA.
4. **CLI Reporting:** Updated UI components to display the RCA summary clearly.

## Success Criteria
- [ ] Investigations produce at least one "Root Cause" finding when evidence is available.
- [ ] Every RCA finding is accompanied by at least two "Suggested Remedies".
- [ ] Automated tests verify the correlation logic with mock investigation data.
- [ ] Code coverage for new RCA components exceeds 80%.