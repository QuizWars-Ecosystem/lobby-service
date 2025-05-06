package report

import "sync"

func countAsyncMap(m *sync.Map) int {
	count := 0
	m.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func cutAsyncMap(m *sync.Map, amount int) *sync.Map {
	newMap := &sync.Map{}

	m.Range(func(k, v interface{}) bool {
		newMap.Store(k, v)
		if amount--; amount == 0 {
			return false
		}
		return true
	})

	return newMap
}
