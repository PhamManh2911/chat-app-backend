package transform

import "strings"

func ExtractTokenFromBearer(token string) string {
	items := strings.Split(token, " ")

	return items[len(items)-1]
}