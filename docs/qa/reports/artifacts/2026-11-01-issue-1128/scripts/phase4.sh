#!/bin/bash
source "$(dirname "${BASH_SOURCE[0]}")/env.sh" || exit 1
cd "$LAB" || exit 1; S2=$(cat $QA/s2.id)
sleep 160
step api monitors-ineligible-owner-after-deadline curl --fail --silent --show-error --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null || exit 1
step cli session-events-ineligible-owner compozy session events $S2 -o json >/dev/null || exit 1
step runtime db-monitors-ineligible-final dbmon >/dev/null || exit 1
step cli session-health-ineligible-owner compozy session health $S2 >/dev/null || exit 1
echo finished4
