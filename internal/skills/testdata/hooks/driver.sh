#!/bin/sh

set -eu

payload=$(cat)

if [ "${HOOK_TEST_BUSY_LOOP:-0}" = "1" ]; then
	while :; do
		:
	done
fi

if [ -n "${HOOK_TEST_STDERR:-}" ]; then
	printf '%s' "${HOOK_TEST_STDERR}" >&2
fi

case "${HOOK_TEST_OUTPUT_MODE:-value}" in
	value)
		printf '%s' "${HOOK_TEST_OUTPUT:-}"
		;;
	payload)
		printf '%s' "$payload"
		;;
	env)
		printf '%s' "${HOOK_TEST_CUSTOM_ENV:-}"
		;;
	combined)
		printf '%s|%s' "$payload" "${HOOK_TEST_CUSTOM_ENV:-}"
		;;
	*)
		printf '%s' "${HOOK_TEST_OUTPUT:-}"
		;;
esac

exit_code="${HOOK_TEST_EXIT_CODE:-0}"
if [ "$exit_code" -ne 0 ]; then
	exit "$exit_code"
fi
