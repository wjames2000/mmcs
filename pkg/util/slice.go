package util

// ContainSlice 检查切片中是否包含目标元素
func ContainSlice[T comparable](elems []T, target T) bool {
	for _, e := range elems {
		if e == target {
			return true
		}
	}
	return false
}

// FilterSlice 过滤切片，保留满足条件的元素
func FilterSlice[T any](elems []T, fn func(T) bool) []T {
	result := make([]T, 0, len(elems))
	for _, e := range elems {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

// MapSlice 对切片中每个元素执行映射
func MapSlice[T any, U any](elems []T, fn func(T) U) []U {
	result := make([]U, len(elems))
	for i, e := range elems {
		result[i] = fn(e)
	}
	return result
}

// ReduceSlice 对切片进行归约操作
func ReduceSlice[T any, U any](elems []T, init U, fn func(U, T) U) U {
	acc := init
	for _, e := range elems {
		acc = fn(acc, e)
	}
	return acc
}

// UniqueSlice 去重切片元素
func UniqueSlice[T comparable](elems []T) []T {
	seen := make(map[T]struct{}, len(elems))
	result := make([]T, 0, len(elems))
	for _, e := range elems {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			result = append(result, e)
		}
	}
	return result
}

// ChunkSlice 将切片按指定大小分块
func ChunkSlice[T any](elems []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	chunks := make([][]T, 0, (len(elems)+size-1)/size)
	for i := 0; i < len(elems); i += size {
		end := i + size
		if end > len(elems) {
			end = len(elems)
		}
		chunks = append(chunks, elems[i:end])
	}
	return chunks
}

// DiffSlice 返回两个切片的差集（在 a 中但不在 b 中）
func DiffSlice[T comparable](a, b []T) []T {
	setB := make(map[T]struct{}, len(b))
	for _, v := range b {
		setB[v] = struct{}{}
	}
	result := make([]T, 0, len(a))
	for _, v := range a {
		if _, ok := setB[v]; !ok {
			result = append(result, v)
		}
	}
	return result
}

// Intersect 返回两个切片的交集
func Intersect[T comparable](a, b []T) []T {
	setB := make(map[T]struct{}, len(b))
	for _, v := range b {
		setB[v] = struct{}{}
	}
	result := make([]T, 0, min(len(a), len(b)))
	for _, v := range a {
		if _, ok := setB[v]; ok {
			result = append(result, v)
		}
	}
	return result
}
