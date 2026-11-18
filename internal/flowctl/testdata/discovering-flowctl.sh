#!/bin/sh
# Fake flowctl for discovery publications: records each call's arguments as [arg][arg] in $FLOWCTL_CALLS and answers:
# - pull-specs of acmeCo/mongodb/shop/orders: $FLOWCTL_PULLED, the live collection to reset
# - other pull-specs: nothing, as Estuary doesn't have those collections yet
# - discover: $FLOWCTL_DISCOVERED, written over the --source spec
# - publish: copies the published spec to $FLOWCTL_PUBLISHED, and its resets file, if any, to $FLOWCTL_PUBLISHED_RESETS

line=""
previous=""

for argument in "$@"; do
	line="$line[$argument]"
	if [ "$previous" = "--target" ]; then target="$argument"; fi
	if [ "$previous" = "--source" ]; then source="$argument"; fi
	previous="$argument"
done

echo "$line" >> "$FLOWCTL_CALLS"

case "$line" in
	*"[--name=acmeCo/mongodb/shop/orders]"*) cat "$FLOWCTL_PULLED" > "$target" ;;
	"[catalog][pull-specs]"*) : > "$target" ;;
	"[discover]"*) cat "$FLOWCTL_DISCOVERED" > "$source" ;;
	"[catalog][publish]"*)
		cp "$source" "$FLOWCTL_PUBLISHED"
		if [ -f "$(dirname "$source")/resets/flow.yaml" ]; then cp "$(dirname "$source")/resets/flow.yaml" "$FLOWCTL_PUBLISHED_RESETS"; fi
		;;
esac
