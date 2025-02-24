#!/bin/sh
WORKER_ID="worker-${HOSTNAME##*-}"
exec ./goload worker --master master:8080 --id "$WORKER_ID"