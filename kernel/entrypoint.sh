#!/bin/sh
set -e

# Default values
PUID=${PUID:-1000}
PGID=${PGID:-1000}
USER_NAME=${USER_NAME:-siyuan}
GROUP_NAME=${GROUP_NAME:-siyuan}
WORKSPACE_DIR="/siyuan/workspace"

# Get or create group
group_name="${GROUP_NAME}"
if getent group "${PGID}" > /dev/null 2>&1; then
    group_name=$(getent group "${PGID}" | cut -d: -f1)
    echo "Using existing group: ${group_name} (${PGID})"
else
    echo "Creating group ${group_name} (${PGID})"
    addgroup --gid "${PGID}" "${group_name}"
fi

# Get or create user
user_name="${USER_NAME}"
if getent passwd "${PUID}" > /dev/null 2>&1; then
    user_name=$(getent passwd "${PUID}" | cut -d: -f1)
    echo "Using existing user ${user_name} (PUID: ${PUID}, PGID: ${PGID})"
else
    echo "Creating user ${user_name} (PUID: ${PUID}, PGID: ${PGID})"
    adduser --uid "${PUID}" --ingroup "${group_name}" --disabled-password --gecos "" "${user_name}"
fi

# Accept flag-only commands from existing Compose deployments.
case "${1:-}" in
    /opt/siyuan/kernel) shift ;;
esac
case "${1:-}" in
    ""|-*) set -- serve "$@" ;;
esac

if [ -n "${SIYUAN_WORKSPACE_PATH}" ]; then
    WORKSPACE_DIR="${SIYUAN_WORKSPACE_PATH}"
fi
for arg in "$@"; do
    case "$arg" in
        --workspace=*) WORKSPACE_DIR="${arg#*=}" ;;
    esac
done

# Change ownership of relevant directories, including the workspace directory
echo "Adjusting ownership of /opt/siyuan, /home/siyuan/, and ${WORKSPACE_DIR}"
chown -R "${PUID}:${PGID}" /opt/siyuan
chown -R "${PUID}:${PGID}" /home/siyuan/
chown -R "${PUID}:${PGID}" "${WORKSPACE_DIR}"

# Switch to the newly created user and start the main process with all arguments
echo "Starting SiYuan with UID:${PUID} and GID:${PGID} in workspace ${WORKSPACE_DIR}"
exec su-exec "${PUID}:${PGID}" /opt/siyuan/kernel --workspace="${WORKSPACE_DIR}" "$@"
