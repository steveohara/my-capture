#!/usr/bin/env bash
#
# next-version.sh prints the tag for the next release of my-capture.
#
# Usage: next-version.sh [minor|patch|major]
#
# It finds the highest existing tag of the form vMAJOR.MINOR.PATCH, such as
# v0.3.1, and increments one part of it:
#
#   minor (the default)  v0.3.1 -> v0.4.0
#   patch                v0.3.1 -> v0.3.2
#   major                v0.3.1 -> v1.0.0
#
# Lower parts are reset to zero. With no existing tag the first release is
# v0.1.0. Tags that are not plain three-part versions, such as v1.0.0-rc1 or
# a tag called "latest", are ignored.
#
# The script only reads tags. It must run in a clone that has fetched all
# tags (actions/checkout with fetch-depth: 0), and it changes nothing.
#
# Environment variables: none.

set -euo pipefail

bump="${1:-minor}"

# Highest version first. --sort=-v:refname compares numerically per part, so
# v0.10.0 correctly ranks above v0.9.0. grep drops pre-release and other
# tags; "|| true" stops a repository with no tags from failing the pipeline.
latest="$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1 || true)"

if [ -z "$latest" ]; then
	echo "v0.1.0"
	exit 0
fi

IFS=. read -r major minor patch <<<"${latest#v}"

case "$bump" in
minor) echo "v${major}.$((minor + 1)).0" ;;
patch) echo "v${major}.${minor}.$((patch + 1))" ;;
major) echo "v$((major + 1)).0.0" ;;
*)
	echo "unknown bump '$bump': use minor, patch or major" >&2
	exit 1
	;;
esac
