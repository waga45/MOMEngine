package core

import (
	"fmt"
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
