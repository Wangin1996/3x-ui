package runtime

import "encoding/json"

// SanitizeStreamSettingsForRemote strips file-based TLS certificate paths from
// the StreamSettings, but ONLY when inline certificate content (certificate /
// key) is also present in the same entry. In that case the file paths are
// redundant and stripping them avoids referencing central-panel paths that do
// not exist on a node's filesystem. Entries with only file paths are left
// untouched. Used by the agent config renderer so a pulled config never points
// at cert files that live only on the master.
func SanitizeStreamSettingsForRemote(streamSettings string) string {
	if streamSettings == "" {
		return streamSettings
	}

	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return streamSettings
	}

	tlsSettings, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return streamSettings
	}

	certificates, ok := tlsSettings["certificates"].([]any)
	if !ok {
		return streamSettings
	}

	changed := false
	for _, cert := range certificates {
		c, ok := cert.(map[string]any)
		if !ok {
			continue
		}
		hasCertFile := c["certificateFile"] != nil && c["certificateFile"] != ""
		hasKeyFile := c["keyFile"] != nil && c["keyFile"] != ""
		hasCertInline := isNonEmptySlice(c["certificate"])
		hasKeyInline := isNonEmptySlice(c["key"])
		if hasCertFile && hasCertInline {
			delete(c, "certificateFile")
			changed = true
		}
		if hasKeyFile && hasKeyInline {
			delete(c, "keyFile")
			changed = true
		}
	}

	if !changed {
		return streamSettings
	}
	out, err := json.Marshal(stream)
	if err != nil {
		return streamSettings
	}
	return string(out)
}

func isNonEmptySlice(v any) bool {
	s, ok := v.([]any)
	return ok && len(s) > 0
}
