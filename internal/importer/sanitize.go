package importer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)
var multiUnderscore = regexp.MustCompile(`_+`)

// SanitizeName converts a key/value pair into a valid Terraform resource name.
// Converts to lowercase, replaces non-alphanumeric characters with underscores,
// collapses consecutive underscores, and trims leading/trailing underscores.
func SanitizeName(key, value string) string {
	// Transliterate common accented characters
	name := strings.ToLower(key + "_" + value)
	name = transliterate(name)
	name = nonAlphaNum.ReplaceAllString(name, "_")
	name = multiUnderscore.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")

	// Ensure name doesn't start with a digit
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	if name == "" {
		name = "resource"
	}

	return name
}

// transliterate replaces common accented characters with ASCII equivalents.
func transliterate(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 'á' || r == 'à' || r == 'â' || r == 'ä' || r == 'ã' || r == 'å':
			b.WriteRune('a')
		case r == 'é' || r == 'è' || r == 'ê' || r == 'ë':
			b.WriteRune('e')
		case r == 'í' || r == 'ì' || r == 'î' || r == 'ï':
			b.WriteRune('i')
		case r == 'ó' || r == 'ò' || r == 'ô' || r == 'ö' || r == 'õ':
			b.WriteRune('o')
		case r == 'ú' || r == 'ù' || r == 'û' || r == 'ü':
			b.WriteRune('u')
		case r == 'ñ':
			b.WriteRune('n')
		case r == 'ç':
			b.WriteRune('c')
		case r > unicode.MaxASCII:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NamedResource holds a resource with its generated Terraform name and original ID.
type NamedResource struct {
	Name string
	ID   string
}

// DeduplicateNames takes a map of ID -> name and returns a deduplicated list
// sorted by ID for determinism. Collisions get _2, _3, etc. suffixes.
func DeduplicateNames(idToName map[string]string) []NamedResource {
	// Sort by ID for determinism
	ids := make([]string, 0, len(idToName))
	for id := range idToName {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	nameCount := make(map[string]int)
	result := make([]NamedResource, 0, len(ids))

	for _, id := range ids {
		name := idToName[id]
		nameCount[name]++
		if nameCount[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, nameCount[name])
		}
		result = append(result, NamedResource{Name: name, ID: id})
	}

	return result
}
