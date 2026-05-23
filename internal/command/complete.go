package command

import "strings"

func completeFirstToken(line, token string, cands []string) (string, []string) {
	matches := filterPrefix(cands, token)
	if len(matches) == 0 {
		return line, nil
	}
	if len(matches) == 1 {
		prefix := strings.TrimSuffix(line, token)
		return prefix + matches[0] + " ", matches
	}
	return line, matches
}

func filterPrefix(cands []string, cur string) []string {
	if cur == "" {
		return append([]string{}, cands...)
	}
	res := make([]string, 0, len(cands))
	for _, c := range cands {
		if strings.HasPrefix(strings.ToLower(c), strings.ToLower(cur)) {
			res = append(res, c)
		}
	}
	return res
}

func completeForward(args []string) []string {
	if len(args) <= 2 {
		return []string{"list", "add", "remove", "enable", "disable", "update"}
	}

	if len(args) == 3 && args[1] == "add" {
		return []string{"tcp", "udp", "tcp-s", "udp-s", "com"}
	}

	if len(args) == 4 && args[1] == "update" {
		return []string{"tcp", "udp", "tcp-s", "udp-s", "com"}
	}

	return nil
}

func completePlugin(args []string) []string {
	if len(args) <= 2 {
		return []string{"list", "load", "unload", "enable", "disable", "reload"}
	}
	return nil
}

func completeMode(args []string) []string {
	if len(args) <= 2 {
		return []string{"show", "set"}
	}

	if len(args) == 3 && args[1] == "set" {
		return []string{"in", "out", "end", "frame", "timestamp", "timefmt"}
	}

	if len(args) == 4 && args[1] == "set" && args[2] == "timestamp" {
		return []string{"on", "off"}
	}

	return nil
}
