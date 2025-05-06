package report

func cutMap[K comparable, V any](source map[K]V, amount int) map[K]V {
	newMap := make(map[K]V, amount)

	for k, v := range source {
		newMap[k] = v
		amount--
		if amount <= 0 {
			return newMap
		}
	}

	return newMap
}
