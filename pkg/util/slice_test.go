package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===== FilterSlice =====

func TestFilterSlice_Ints(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := FilterSlice(nums, func(n int) bool { return n%2 == 0 })
	assert.Equal(t, []int{2, 4, 6}, evens)
}

func TestFilterSlice_Strings(t *testing.T) {
	strs := []string{"a", "ab", "abc", "abcd", "abcde"}
	longer := FilterSlice(strs, func(s string) bool { return len(s) > 3 })
	assert.Equal(t, []string{"abcd", "abcde"}, longer)
}

func TestFilterSlice_Empty(t *testing.T) {
	result := FilterSlice([]int{}, func(n int) bool { return n > 0 })
	assert.Empty(t, result)
}

func TestFilterSlice_AllMatch(t *testing.T) {
	nums := []int{2, 4, 6}
	result := FilterSlice(nums, func(n int) bool { return n%2 == 0 })
	assert.Equal(t, []int{2, 4, 6}, result)
}

func TestFilterSlice_NoneMatch(t *testing.T) {
	nums := []int{1, 3, 5}
	result := FilterSlice(nums, func(n int) bool { return n%2 == 0 })
	assert.Empty(t, result)
}

// ===== MapSlice =====

func TestMapSlice_IntToString(t *testing.T) {
	nums := []int{1, 2, 3}
	strs := MapSlice(nums, func(n int) string {
		return string(rune('A' + n - 1))
	})
	assert.Equal(t, []string{"A", "B", "C"}, strs)
}

func TestMapSlice_Transform(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}
	doubled := MapSlice(nums, func(n int) int { return n * 2 })
	assert.Equal(t, []int{2, 4, 6, 8, 10}, doubled)
}

func TestMapSlice_Empty(t *testing.T) {
	result := MapSlice([]int{}, func(n int) string { return "" })
	assert.Empty(t, result)
}

// ===== ReduceSlice =====

func TestReduceSlice_Sum(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}
	sum := ReduceSlice(nums, 0, func(acc, n int) int { return acc + n })
	assert.Equal(t, 15, sum)
}

func TestReduceSlice_Concat(t *testing.T) {
	strs := []string{"a", "b", "c"}
	result := ReduceSlice(strs, "", func(acc, s string) string { return acc + s })
	assert.Equal(t, "abc", result)
}

func TestReduceSlice_Empty(t *testing.T) {
	result := ReduceSlice([]int{}, 42, func(acc, n int) int { return acc + n })
	assert.Equal(t, 42, result)
}

// ===== UniqueSlice =====

func TestUniqueSlice_Ints(t *testing.T) {
	nums := []int{1, 2, 2, 3, 3, 3, 4, 5, 5}
	unique := UniqueSlice(nums)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, unique)
}

func TestUniqueSlice_Strings(t *testing.T) {
	strs := []string{"a", "b", "a", "c", "b", "d"}
	unique := UniqueSlice(strs)
	assert.Equal(t, []string{"a", "b", "c", "d"}, unique)
}

func TestUniqueSlice_Empty(t *testing.T) {
	result := UniqueSlice([]int{})
	assert.Empty(t, result)
}

func TestUniqueSlice_AlreadyUnique(t *testing.T) {
	nums := []int{1, 2, 3}
	result := UniqueSlice(nums)
	assert.Equal(t, []int{1, 2, 3}, result)
}

// ===== ChunkSlice =====

func TestChunkSlice_Even(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6}
	chunks := ChunkSlice(nums, 2)
	assert.Equal(t, 3, len(chunks))
	assert.Equal(t, []int{1, 2}, chunks[0])
	assert.Equal(t, []int{3, 4}, chunks[1])
	assert.Equal(t, []int{5, 6}, chunks[2])
}

func TestChunkSlice_Uneven(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}
	chunks := ChunkSlice(nums, 2)
	assert.Equal(t, 3, len(chunks))
	assert.Equal(t, []int{1, 2}, chunks[0])
	assert.Equal(t, []int{3, 4}, chunks[1])
	assert.Equal(t, []int{5}, chunks[2])
}

func TestChunkSlice_Smaller(t *testing.T) {
	nums := []int{1, 2, 3}
	chunks := ChunkSlice(nums, 10)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, []int{1, 2, 3}, chunks[0])
}

func TestChunkSlice_Empty(t *testing.T) {
	chunks := ChunkSlice([]int{}, 3)
	assert.Empty(t, chunks)
}

func TestChunkSlice_SizeOne(t *testing.T) {
	nums := []int{1, 2, 3}
	chunks := ChunkSlice(nums, 1)
	assert.Equal(t, 3, len(chunks))
	assert.Equal(t, []int{1}, chunks[0])
	assert.Equal(t, []int{2}, chunks[1])
	assert.Equal(t, []int{3}, chunks[2])
}

func TestChunkSlice_InvalidSize(t *testing.T) {
	nums := []int{1, 2, 3}
	assert.Nil(t, ChunkSlice(nums, 0))
	assert.Nil(t, ChunkSlice(nums, -1))
}

// ===== DiffSlice =====

func TestDiffSlice_Basic(t *testing.T) {
	a := []int{1, 2, 3, 4, 5}
	b := []int{2, 4}
	diff := DiffSlice(a, b)
	assert.Equal(t, []int{1, 3, 5}, diff)
}

func TestDiffSlice_NoDiff(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}
	diff := DiffSlice(a, b)
	assert.Empty(t, diff)
}

func TestDiffSlice_AllDiff(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{4, 5, 6}
	diff := DiffSlice(a, b)
	assert.Equal(t, []int{1, 2, 3}, diff)
}

func TestDiffSlice_EmptyA(t *testing.T) {
	diff := DiffSlice([]int{}, []int{1, 2, 3})
	assert.Empty(t, diff)
}

func TestDiffSlice_EmptyB(t *testing.T) {
	a := []int{1, 2, 3}
	diff := DiffSlice(a, []int{})
	assert.Equal(t, []int{1, 2, 3}, diff)
}

// ===== Intersect =====

func TestIntersect_Basic(t *testing.T) {
	a := []int{1, 2, 3, 4, 5}
	b := []int{2, 4, 6}
	result := Intersect(a, b)
	assert.Equal(t, []int{2, 4}, result)
}

func TestIntersect_NoOverlap(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{4, 5, 6}
	result := Intersect(a, b)
	assert.Empty(t, result)
}

func TestIntersect_AllOverlap(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}
	result := Intersect(a, b)
	assert.Equal(t, []int{1, 2, 3}, result)
}

func TestIntersect_Empty(t *testing.T) {
	result := Intersect([]int{}, []int{1, 2, 3})
	assert.Empty(t, result)
}

func TestIntersect_Duplicates(t *testing.T) {
	a := []int{1, 1, 2, 3}
	b := []int{1, 2, 2, 4}
	result := Intersect(a, b)
	// Intersect 保留 a 中重复元素（仅检查在 b 中的存在性）
	assert.Equal(t, []int{1, 1, 2}, result)
}

func TestIntersect_DuplicatesInBoth(t *testing.T) {
	a := []int{1, 1, 2, 2}
	b := []int{1, 1, 1, 3}
	result := Intersect(a, b)
	assert.Equal(t, []int{1, 1}, result)
}

// ===== ContainSlice =====

func TestContainSlice_Found(t *testing.T) {
	assert.True(t, ContainSlice([]int{1, 2, 3}, 2))
	assert.True(t, ContainSlice([]string{"a", "b", "c"}, "b"))
}

func TestContainSlice_NotFound(t *testing.T) {
	assert.False(t, ContainSlice([]int{1, 2, 3}, 4))
	assert.False(t, ContainSlice([]string{"a", "b", "c"}, "d"))
}

func TestContainSlice_Empty(t *testing.T) {
	assert.False(t, ContainSlice([]int{}, 1))
}
