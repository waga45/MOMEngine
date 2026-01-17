package core

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/quagmt/udecimal"
)

func TestSkipListBase(t *testing.T) {
	sl := NewSkipList(10, 4, false)
	for i := int64(1); i < 10; i++ {
		sl.Insert(udecimal.MustFromInt64(i, 0))
	}
	fmt.Println(sl.GetValueSlice())

	for i := int32(0); i < sl.level; i++ {
		fmt.Println("level:", i+1)
		fmt.Println(sl.LevelNodes(i))
	}

	sl.RemoveMin()
	fmt.Println(sl.GetValueSlice())

	sl.Remove(udecimal.MustFromInt64(6, 0))
	fmt.Println(sl.GetValueSlice())

	for i := int32(0); i < sl.level; i++ {
		fmt.Println("level:", i+1)
		fmt.Println(sl.LevelNodes(i))
	}
}

// TestSkipListLargeData 在大数据量场景下验证跳表的正确性和稳定性
func TestSkipListLargeData(t *testing.T) {
	const n = 100000

	sl := NewSkipList(int32(n), 1, false)

	for i := int64(1); i <= n; i++ {
		ok, err := sl.Insert(udecimal.MustFromInt64(i, 0))
		if err != nil {
			t.Fatalf("insert error at %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("unexpected duplicate insert at %d", i)
		}
	}

	if sl.Count() != int32(n) {
		t.Fatalf("count mismatch: got %d, want %d", sl.Count(), n)
	}

	values := sl.GetValueSlice()
	if len(values) != int(n) {
		t.Fatalf("value slice length mismatch: got %d, want %d", len(values), n)
	}
	for i := 1; i < len(values); i++ {
		if values[i].LessThan(values[i-1]) {
			t.Fatalf("values not sorted at %d", i)
		}
	}

	r := rand.New(rand.NewSource(1))
	removed := 0
	targetRemoved := n / 2

	for removed < targetRemoved {
		v := int64(r.Intn(n) + 1)
		ok, err := sl.Remove(udecimal.MustFromInt64(v, 0))
		if err != nil {
			t.Fatalf("remove error: %v", err)
		}
		if ok {
			removed++
		}
	}

	if sl.Count() != int32(n-removed) {
		t.Fatalf("count mismatch after remove: got %d, want %d", sl.Count(), n-removed)
	}

	values = sl.GetValueSlice()
	for i := 1; i < len(values); i++ {
		if values[i].LessThan(values[i-1]) {
			t.Fatalf("values not sorted after remove at %d", i)
		}
	}
}

// BenchmarkSkipListInsertSequential 顺序插入的性能基准测试
func BenchmarkSkipListInsertSequential(b *testing.B) {
	sl := NewSkipList(int32(b.N), 1, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sl.Insert(udecimal.MustFromInt64(int64(i), 0)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSkipListInsertRandom 随机插入的性能基准测试
func BenchmarkSkipListInsertRandom(b *testing.B) {
	sl := NewSkipList(int32(b.N), 1, false)
	r := rand.New(rand.NewSource(1))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := int64(r.Int())
		if _, err := sl.Insert(udecimal.MustFromInt64(v, 0)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSkipListContainsHit 命中场景下的 Contains 查询性能基准测试
func BenchmarkSkipListContainsHit(b *testing.B) {
	const n = 100000

	sl := NewSkipList(int32(n), 1, false)
	for i := int64(1); i <= n; i++ {
		if _, err := sl.Insert(udecimal.MustFromInt64(i, 0)); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := int64(i%int(n) + 1)
		found, _ := sl.Contains(udecimal.MustFromInt64(key, 0))
		if !found {
			b.Fatalf("expected hit for key %d", key)
		}
	}
}

// BenchmarkSkipListContainsMiss 未命中场景下的 Contains 查询性能基准测试
func BenchmarkSkipListContainsMiss(b *testing.B) {
	const n = 1000000

	sl := NewSkipList(int32(n), 1, false)
	for i := int64(1); i <= n; i++ {
		if _, err := sl.Insert(udecimal.MustFromInt64(i, 0)); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := int64(n) + 1 + int64(i%10)
		found, _ := sl.Contains(udecimal.MustFromInt64(key, 0))
		if found {
			b.Fatalf("unexpected hit for key %d", key)
		}
	}
}
