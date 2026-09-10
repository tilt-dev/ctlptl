#!/bin/bash
# Helpers for checking that `ctlptl apply` is idempotent.
#
# Usage:
#   REGISTRY_ID=$(registry_id ctlptl-test-registry)
#   ctlptl apply -f registry.yaml
#   assert_unchanged "registry" "$REGISTRY_ID" "$(registry_id ctlptl-test-registry)"

# The container backing a registry. Changes if the registry is recreated.
registry_id() {
    ctlptl get registry "$1" -o template --template '{{.status.containerId}}'
}

# The creation time of a cluster's oldest node. Changes if the cluster is recreated.
cluster_id() {
    ctlptl get cluster "$1" -o template --template '{{.status.creationTimestamp}}'
}

# assert_unchanged DESCRIPTION BEFORE AFTER
assert_unchanged() {
    local desc="$1"
    local before="$2"
    local after="$3"

    if [[ "$before" == "" ]]; then
        echo "could not read the identity of $desc before re-applying"
        exit 1
    fi

    if [[ "$before" != "$after" ]]; then
        echo "$desc was recreated by a second 'ctlptl apply' (before: $before, after: $after)"
        exit 1
    fi
}
