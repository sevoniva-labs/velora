package bootstrap

import "strings"

func hasTopic(topics []string, target string) bool {
	for _, topic := range topics {
		if strings.TrimSpace(topic) == target {
			return true
		}
	}
	return false
}
