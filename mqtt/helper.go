package mqtt

import "strings"

func getTopicPart(topic string, idx int) string {
	s := strings.Split(topic, "/")
	switch true {
	case idx > 0 && idx < len(s):
		return s[idx]
	case idx < 0 && idx >= -len(s):
		return s[len(s)+idx]
	default:
		return ""
	}
}

func findInJSON(jsonMap map[string]interface{}, path string) (interface{}, bool) {
	if path == "" || len(jsonMap) == 0 {
		return nil, false
	}

	// First try to find with original path (dots)
	if val, ok := tryFind(jsonMap, path, false); ok {
		return val, true
	}

	// If not found, try with underscores
	if val, ok := tryFind(jsonMap, path, true); ok {
		return val, true
	}

	return nil, false
}

func tryFind(jsonMap map[string]interface{}, path string, replaceDots bool) (interface{}, bool) {
	searchPath := path
	if replaceDots {
		searchPath = strings.ReplaceAll(path, ".", "_")
	}

	pp := strings.SplitN(searchPath, ".", 2)
	if val, found := jsonMap[pp[0]]; found && len(pp) > 1 {
		if subJSONMap, ok := val.(map[string]interface{}); ok {
			return tryFind(subJSONMap, pp[1], replaceDots)
		}
		return nil, false
	} else if found {
		return val, true
	}
	return nil, false
}
