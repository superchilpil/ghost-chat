#!/usr/bin/env bash
set -euo pipefail

rc=false
override=""

while [ $# -gt 0 ]; do
  case "$1" in
    --rc)
      rc=true
      ;;
    --override)
      if [ -z "${2:-}" ]; then
        echo "--override requires a value" >&2
        exit 1
      fi

      override="$2"
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
  shift
done

if [ -n "$override" ]; then
  echo "$override"
  exit 0
fi

# Accept both v1.2.3 and 1.2.3 tags. Manual release overrides have historically
# used the no-v prefix, so automatic versioning must recognize both forms.
last_stable_tag="$(
  git tag --list |
    grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' |
    sort -V |
    tail -n 1 || true
)

if [ -z "$last_stable_tag" ]; then
  # No release tag yet: start at 0.0.0 so the first automatic release is 0.0.1.
  last_stable_tag="0.0.0"
  last_stable="0.0.0"
  subjects="$(git log --format=%s)"
  bodies="$(git log --format=%b)"
else
  last_stable="$last_stable_tag"
  range="${last_stable_tag}..HEAD"
  subjects="$(git log --format=%s "$range")"
  bodies="$(git log --format=%b "$range")"
fi


if [ -z "$subjects" ]; then
  echo "no commits since ${last_stable}" >&2
  exit 1
fi

bump=patch

if echo "$subjects" | grep -qE '^[a-z]+(\(.+\))?!:' || echo "$bodies" | grep -qE '^BREAKING[ -]CHANGE: '; then
  bump=major
elif echo "$subjects" | grep -qE '^feat(\(.+\))?:'; then
  bump=minor
fi

IFS=. read -r major minor patch <<<"${last_stable#v}"

case "$bump" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
esac

# Treat the numeric components as single decimal digits. Carry after 9:
#   1.0.9 -> 1.1.0
#   1.9.9 -> 2.0.0
if [ "$patch" -gt 9 ]; then
  minor=$((minor + patch / 10))
  patch=$((patch % 10))
fi

if [ "$minor" -gt 9 ]; then
  major=$((major + minor / 10))
  minor=$((minor % 10))
fi

prefix=""
if [[ "$last_stable_tag" == v* ]]; then
  prefix="v"
fi

next="${prefix}${major}.${minor}.${patch}"

if [ "$rc" = true ]; then
  last_rc=$(git tag --list "${next}-rc.*" | sed 's/.*-rc\.//' | sort -n | tail -n 1)
  next="${next}-rc.$((${last_rc:-0} + 1))"
fi

echo "$next"
