#!/bin/bash

# Function to send a log message
send_log() {
    level=$1
    message=$2
    context=$3
    nats pub "compozy.log.$level" "{\"type\":\"Log\",\"payload\":{\"log_level\":\"$level\",\"message\":\"$message\",\"context\":$context,\"timestamp\":\"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\"}}"
}

# Send test messages
send_log "debug" "Debug test message" '{"test":"debug","value":1}'
send_log "info" "Info test message" '{"test":"info","value":2}'
send_log "warn" "Warning test message" '{"test":"warn","value":3}'
send_log "error" "Error test message" '{"test":"error","value":4}'