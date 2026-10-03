#!/usr/bin/env bash
set -euo pipefail

rc=false
override=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --rc) rc=true ;;
    --override)
      if [ "$#" -lt 2 ]; then echo "--override requires a value" >&2; exit 1; fi
      override="$2"
      shift ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
  shift
done

if [ -n "$override" ]; then echo "$override"; exit 0; fi

last_stable_tag=$(git tag --list | grep -E "^v?[0-9]+\.[0-9]+\.[0-9]+$" | sort -V | tail -n 1 || true)

if [ -z "$last_stable_tag" ]; then
  last_stable_tag="0.0.0"
  subjects=$(git log --format=%s)
  bodies=$(git log --format=%b)
else
  range="$last_stable_tag..HEAD"
  subjects=$(git log --format=%s "$range")
  bodies=$(git log --format=%b "$range")
fi

if [ -z "$subjects" ]; then echo "no commits since $last_stable_tag" >&2; exit 1; fi

bump="patch"
if printf "%s\n" "$subjects" | grep -qE "^[a-z]+(\(.+\))?!:"; then
  bump="major"
elif printf "%s\n" "$bodies" | grep -qE "^BREAKING[ -]CHANGE: "; then
  bump="major"
elif printf "%s\n" "$subjects" | grep -qE "^feat(\(.+\))?: "; then
  bump="minor"
fi

version="${last_stable_tag#v}"
IFS=. read -r major minor patch <<< "$version"

case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac

if [ "$patch" -ge 10 ]; then patch=0; minor=$((minor + 1)); fi
if [ "$minor" -ge 10 ]; then minor=0; major=$((major + 1)); fi

prefix=""
case "$last_stable_tag" in v*) prefix="v" ;; esac
next="${prefix}${major}.${minor}.${patch}"

if [ "$rc" = true ]; then
  last_rc_tag=$(git tag --list "${next}-rc.*" | sort -V | tail -n 1 || true)
  if [ -n "$last_rc_tag" ]; then
    last_rc="${last_rc_tag##*-rc.}"
  else
    last_rc=0
  fi
  next="${next}-rc.$((last_rc + 1))"
fi

echo "$next"
