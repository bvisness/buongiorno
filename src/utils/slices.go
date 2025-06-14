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

func AppendToSliceIfAbsent[T any, K comparable](items *[]T, newItem T, f func(T) K) *T {
	key := f(newItem)
	for i := range *items {
		if key == f((*items)[i]) {
			return &(*items)[i]
		}
	}
	*items = append(*items, newItem)
	return &(*items)[len(*items)-1]
}

func UpsertIntoSlice[T any, K comparable](items *[]T, newItem T, f func(T) K) {
	key := f(newItem)
	for i := range *items {
		if key == f((*items)[i]) {
			(*items)[i] = newItem
			return
		}
	}
	*items = append(*items, newItem)
}

func FindInSlice[T any](items []T, f func(T) bool) (*T, bool) {
	for i := range items {
		if f(items[i]) {
			return &items[i], true
		}
	}
	return nil, false
}
