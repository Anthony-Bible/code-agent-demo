# Specification: Improve RCA Findings Display

## Problem Statement
The current implementation of Root Cause Analysis (RCA) findings display is basic and lacks robust integration. `DisplayRCAFindings` is called with ignored errors, and the visual output could be more informative and better structured.

## Goals
- Enhance visual structure of RCA findings in the CLI.
- Standardize error handling for RCA reporting.
- Improve integration between `InvestigationRunner` and `RCAService`.

## Requirements
- Clear visual separation of "Summary", "Causes", and "Remedies".
- Proper error logging/handling for display failures.
- Robust correlation logic handling edge cases (e.g., empty findings).
