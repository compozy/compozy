package desktoprelease

import "encoding/json"

func canonicalJSON(value any) ([]byte, error) {
	return json.Marshal(value)
}
