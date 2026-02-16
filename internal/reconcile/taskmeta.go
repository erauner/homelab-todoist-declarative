package reconcile

import (
	"sort"
	"strings"
)

const managedTaskKeyPrefix = "HTD_KEY:"

func buildManagedTaskDescription(userDescription *string, key string) *string {
	base := ""
	if userDescription != nil {
		base = strings.TrimSpace(*userDescription)
	}
	if key == "" {
		if userDescription == nil {
			return nil
		}
		v := base
		return &v
	}
	if base == "" {
		v := managedTaskKeyPrefix + key
		return &v
	}
	v := base + "\n" + managedTaskKeyPrefix + key
	return &v
}

func taskDescriptionSansManagedKey(description string) string {
	var out []string
	for _, ln := range strings.Split(description, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, managedTaskKeyPrefix) {
			continue
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func equalStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
