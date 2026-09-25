#!/usr/bin/env bash
set -euo pipefail
case "${1:-check}" in
    check|correct) ;;
    *) echo "Error: lint mode must be check or correct" >&2; exit 1 ;;
esac
ineffassign ./...
