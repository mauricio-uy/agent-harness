#!/usr/bin/env python3
"""Validate specifications and regenerate their status-grouped indexes."""

from document_sync import run

TYPES = {
    "adr": ("ADR", "decisions", "Architecture Decision Records", "accepted"),
    "use-case": ("UC", "use-cases", "Use Cases", "approved"),
    "functional-requirement": ("FR", "requirements/functional", "Functional Requirements", "approved"),
    "non-functional-requirement": ("NFR", "requirements/non-functional", "Non-Functional Requirements", "approved"),
}

if __name__ == "__main__":
    raise SystemExit(run(TYPES, "sync-specifications.py", __doc__))
