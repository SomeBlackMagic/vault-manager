package vaultsync

import (
	"sort"
	"strings"

	fmt "github.com/jhunt/go-ansi"
)

// ComputeChanges compares local state vs remote state and returns a ChangeSet.
func ComputeChanges(local []LocalSecret, remote map[string]map[string]interface{}) ChangeSet {
	var changes []Change

	localMap := make(map[string]map[string]interface{}, len(local))
	for _, ls := range local {
		localMap[ls.Path] = ls.Data
	}

	// Collect all paths
	allPaths := make(map[string]bool)
	for _, ls := range local {
		allPaths[ls.Path] = true
	}
	for p := range remote {
		allPaths[p] = true
	}

	sorted := make([]string, 0, len(allPaths))
	for p := range allPaths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	for _, path := range sorted {
		localData, localExists := localMap[path]
		remoteData, remoteExists := remote[path]

		switch {
		case localExists && !remoteExists:
			changes = append(changes, Change{
				Type:      ChangeAdd,
				Path:      path,
				LocalData: localData,
			})
		case !localExists && remoteExists:
			changes = append(changes, Change{
				Type:       ChangeDelete,
				Path:       path,
				RemoteData: remoteData,
			})
		case localExists && remoteExists:
			if mapsEqual(localData, remoteData) {
				changes = append(changes, Change{
					Type:       ChangeNone,
					Path:       path,
					LocalData:  localData,
					RemoteData: remoteData,
				})
			} else {
				changes = append(changes, Change{
					Type:       ChangeModify,
					Path:       path,
					LocalData:  localData,
					RemoteData: remoteData,
				})
			}
		}
	}

	return ChangeSet{Changes: changes}
}

// mapsEqual compares two map[string]interface{} values deeply.
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !ValuesEqual(va, vb) {
			return false
		}
	}
	return true
}

// FormatDiff returns a colored string showing key-level diff for a single Change.
// For keys whose values are nested JSON objects, uses DeepDiffJSON to show
// only the changed fields within the object.
func FormatDiff(c Change) string {
	var sb strings.Builder

	switch c.Type {
	case ChangeAdd:
		sb.WriteString(fmt.Sprintf("@G{+ %s}\n", c.Path))
		keys := sortedKeys(c.LocalData)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("    @G{+ %s}: %s\n", k, formatValue(c.LocalData[k])))
		}

	case ChangeDelete:
		sb.WriteString(fmt.Sprintf("@R{- %s}\n", c.Path))
		keys := sortedKeys(c.RemoteData)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("    @R{- %s}: %s\n", k, formatValue(c.RemoteData[k])))
		}

	case ChangeModify:
		sb.WriteString(fmt.Sprintf("@Y{~ %s}\n", c.Path))
		allKeys := mergedKeys(c.LocalData, c.RemoteData)
		for _, k := range allKeys {
			localVal, localHas := c.LocalData[k]
			remoteVal, remoteHas := c.RemoteData[k]

			if !remoteHas {
				// Key only in local (added)
				sb.WriteString(fmt.Sprintf("    @G{+ %s}: %s\n", k, formatValue(localVal)))
			} else if !localHas {
				// Key only in remote (deleted)
				sb.WriteString(fmt.Sprintf("    @R{- %s}: %s\n", k, formatValue(remoteVal)))
			} else if !ValuesEqual(localVal, remoteVal) {
				// Key in both but differs
				sb.WriteString(formatKeyDiff(k, remoteVal, localVal))
			}
		}

	case ChangeNone:
		sb.WriteString(fmt.Sprintf("  %s\n", c.Path))
	}

	return sb.String()
}

// formatKeyDiff formats a single key diff, with nested JSON support.
func formatKeyDiff(key string, oldVal, newVal interface{}) string {
	var sb strings.Builder

	// Check if both values are structured (map or slice) for nested diff
	_, oldIsMap := oldVal.(map[string]interface{})
	_, newIsMap := newVal.(map[string]interface{})
	_, oldIsSlice := oldVal.([]interface{})
	_, newIsSlice := newVal.([]interface{})

	if (oldIsMap && newIsMap) || (oldIsSlice && newIsSlice) {
		sb.WriteString(fmt.Sprintf("    @Y{~ %s}:\n", key))
		fieldChanges := DeepDiffJSON(oldVal, newVal, "")
		for _, fc := range fieldChanges {
			if fc.OldValue == nil {
				sb.WriteString(fmt.Sprintf("        @G{+ %s}: %s\n", fc.Path, formatValue(fc.NewValue)))
			} else if fc.NewValue == nil {
				sb.WriteString(fmt.Sprintf("        @R{- %s}: %s\n", fc.Path, formatValue(fc.OldValue)))
			} else {
				sb.WriteString(fmt.Sprintf("        @Y{~ %s}:\n", fc.Path))
				sb.WriteString(fmt.Sprintf("            @R{- %s}\n", formatValue(fc.OldValue)))
				sb.WriteString(fmt.Sprintf("            @G{+ %s}\n", formatValue(fc.NewValue)))
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("    @Y{~ %s}:\n", key))
		sb.WriteString(fmt.Sprintf("        @R{- %s}\n", formatValue(oldVal)))
		sb.WriteString(fmt.Sprintf("        @G{+ %s}\n", formatValue(newVal)))
	}

	return sb.String()
}

// FormatChangeSummary returns "Plan: X to add, Y to change, Z to destroy."
func FormatChangeSummary(cs ChangeSet) string {
	adds, modifies, deletes := cs.Counts()
	return fmt.Sprintf("Plan: @G{%d to add}, @Y{%d to change}, @R{%d to destroy}.", adds, modifies, deletes)
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mergedKeys(a, b map[string]interface{}) []string {
	all := make(map[string]bool)
	for k := range a {
		all[k] = true
	}
	for k := range b {
		all[k] = true
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%v", val)
	}
}
