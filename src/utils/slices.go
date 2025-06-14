package utils

func GroupIntoMap[T any, K comparable](items []T, f func(T) K) map[K][]T {
	res := make(map[K][]T)
	for _, item := range items {
		key := f(item)
		res[key] = append(res[key], item)
	}
	return res
}

type SliceGroup[T, K any] struct {
	Key   K
	Items []T
}

func GroupIntoSlice[T any, K comparable](items []T, f func(T) K) []SliceGroup[T, K] {
	var res []SliceGroup[T, K]
nextitem:
	for _, item := range items {
		key := f(item)
		for i := range res {
			group := &res[i]
			if group.Key == key {
				group.Items = append(group.Items, item)
				continue nextitem
			}
		}
		res = append(res, SliceGroup[T, K]{
			Key:   key,
			Items: []T{item},
		})
	}
	return res
}
