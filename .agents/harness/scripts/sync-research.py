#!/usr/bin/env python3
"""Validate research records and regenerate only the research index."""

from document_sync import run

TYPES = {
    "research": ("RES", "research", "Research", "approved"),
}

if __name__ == "__main__":
    raise SystemExit(run(TYPES, "sync-research.py", __doc__))
