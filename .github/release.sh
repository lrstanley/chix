#!/bin/bash

export BASE_MODULE="github.com/lrstanley/chix/v2"

if ! command -v svu >/dev/null 2>&1; then
    echo "svu is not installed"
    exit 1
fi

if [ "$(git rev-parse --show-toplevel)" != "$PWD" ]; then
    echo "--> must be run from the root of the repository"
    exit 1
fi

export CURRENT=$(svu current --pattern "v*")
export NEXT="$1"
if [ -z "$NEXT" ]; then
    export NEXT=$(svu patch --pattern "v*")
fi

if [ "$CURRENT" == "$NEXT" ]; then
    echo "!!> ${CURRENT} is already the latest version"
fi

echo "--> ${CURRENT} -> ${NEXT}"
echo "press enter to continue"
read -r

if ! git tag -l | grep -q "^${NEXT}$"; then
    echo "--> tagging ${NEXT}"
    git tag -m "$NEXT" "$NEXT"
fi

if ! git ls-remote --tags origin | grep -q "/${NEXT}$"; then
    echo "--> pushing ${NEXT}"
    git push origin "$NEXT"
fi

MODULES="$(find "$PWD" -mindepth 2 -maxdepth 3 -name "go.mod" -exec dirname "{}" \;)"

for BASE in $MODULES; do
    MODULE_NAME="$(basename "$BASE")"
    echo "--> updating ${GOMOD}"
    go mod edit -require="${BASE_MODULE}@${NEXT}" "${BASE}/go.mod" || exit 1
    pushd "$BASE" || exit 1
    go mod tidy || exit 1
    echo "--> adding ${BASE}/go.mod & ${BASE}/go.sum"
    git add go.mod go.sum || exit 1
    popd || exit 1
done

# if staged files, commit and push
if ! git diff --cached --quiet >/dev/null; then
    echo "--> committing and pushing go.mod changes"
    git commit -m "chore(release): bump sub-modules to ${NEXT}"
    git push origin
fi

for BASE in $MODULES; do
    MODULE_NAME="$(basename "$BASE")"
    echo "--> tagging ${MODULE_NAME}/${NEXT}"
    if git ls-remote --tags origin | grep -q "/${MODULE_NAME}/${NEXT}$"; then
        echo "!!> ${MODULE_NAME}/${NEXT} already tagged"
        continue
    fi
    git tag -m "${MODULE_NAME}/${NEXT}" "${MODULE_NAME}/${NEXT}" || exit 1
done

git push origin --tags
