#!/bin/bash
source <lab>/scripts/env.sh
cd $LAB; WS=ws_93d7060986409c74; S2=$(cat $QA/s2.id)
sleep 160
step api monitors-ineligible-owner-after-deadline curl -s --unix-socket $COMPOZY_UDS_PATH "http://unix/api/workspaces/$WS/sessions/$S2/monitors" >/dev/null
step cli session-events-ineligible-owner compozy session events $S2 -o json >/dev/null
step runtime db-monitors-ineligible-final dbmon >/dev/null
step cli session-health-ineligible-owner compozy session health $S2 >/dev/null
echo finished4
