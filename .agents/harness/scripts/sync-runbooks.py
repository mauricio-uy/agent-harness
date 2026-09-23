#!/usr/bin/env python3
"""Validate runbook metadata and regenerate only the runbook index."""

from document_metadata import iso_date, positive_int
from document_sync import index_content, run, validate

TYPES = {"runbook": ("RUN", "runbooks", "Runbooks", "approved")}
STATES = ("awaiting-approval", "draft", "approved", "rejected", "retired", "superseded")
VALIDATION_STATES = ("not-validated", "partial", "passed", "failed", "stale")


def single_line(value):
    return isinstance(value, str) and bool(value.strip()) and not any(c in value for c in "\r\n")


def validate_runbook(path, data, kind, types):
    errors = validate(path, data, kind, types, allowed_states=STATES)
    if "owner" not in data:
        errors.append("missing field: owner")
    owner = data.get("owner")
    if owner is None:
        if data.get("status") != "draft":
            errors.append("owner is required outside draft")
    elif not single_line(owner):
        errors.append("owner must be a nonempty single-line string or null in draft")

    validation = data.get("validation")
    required = {"status", "revision", "date", "environment"}
    if not isinstance(validation, dict) or not required <= validation.keys():
        errors.append("validation must contain status, revision, date, and environment")
        return errors
    status = validation["status"]
    if status not in VALIDATION_STATES:
        errors.append("validation.status must be one of: " + ", ".join(VALIDATION_STATES))
        return errors
    if status == "not-validated":
        if any(validation[key] is not None for key in ("revision", "date", "environment")):
            errors.append("not-validated requires null validation revision, date, and environment")
        return errors

    validated, revision = validation["revision"], data.get("revision")
    if not positive_int(validated):
        errors.append("validation.revision must be a positive integer")
    elif positive_int(revision):
        if validated > revision:
            errors.append("validation.revision must not exceed revision")
        elif status != "stale" and validated != revision:
            errors.append(f"validation.status {status} requires the current revision; use stale for older evidence")
    when = iso_date(validation["date"])
    created, updated = iso_date(data.get("created")), iso_date(data.get("updated"))
    if when is None:
        errors.append("validation.date must be a valid YYYY-MM-DD date")
    elif created and updated and not created <= when <= updated:
        errors.append("validation.date must fall between created and updated")
    if not single_line(validation["environment"]):
        errors.append("validation.environment must be a nonempty single-line string")
    return errors


def index_runbooks(kind, records, directory, types, generator):
    return index_content(kind, records, directory, types, generator, allowed_states=STATES,
                         extra_columns=(
                             ("Validation", lambda data: data["validation"]["status"]),
                             ("Validated on", lambda data: iso_date(data["validation"]["date"]).isoformat()
                              if data["validation"]["date"] is not None else "—"),
                         ))


if __name__ == "__main__":
    raise SystemExit(run(TYPES, "sync-runbooks.py", __doc__, validator=validate_runbook, indexer=index_runbooks))
