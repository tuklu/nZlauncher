package main

type stat struct {
	Hits int   `json:"hits"`
	Last int64 `json:"last"`
}

// Zoxide-style frecency: raw hit count weighted by how recently it was used.
func score(s stat, now int64) float64 {
	age := now - s.Last
	weight := 0.25
	switch {
	case age < 3600:
		weight = 4
	case age < 86400:
		weight = 2
	case age < 604800:
		weight = 1
	}
	return float64(s.Hits) * weight
}
