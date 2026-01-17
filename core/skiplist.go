package core

import (
	"MOMEngine/protocol"
	"errors"
	"math"
	"math/rand"

	"github.com/quagmt/udecimal"
)

const (
	MaxLevel        = 16
	RandomLevelRate = 4
	ScaleFactor     = 2
	MaxCapacity     = math.MaxInt32
)

type SkipListNode struct {
	Price   udecimal.Decimal
	Level   int32
	Forward [MaxLevel]int32
}

type SkipList struct {
	nodes      []SkipListNode
	count      int32
	level      int32
	head       int32
	freeHead   int32
	descending bool
	rd         *rand.Rand
}
type SkipListIterator struct {
	sl      *SkipList
	current int32
}

/**
 * Instance New SkipList
 */
func NewSkipList(capacity int32, seed int64, descending bool) *SkipList {
	if capacity <= 0 || capacity > MaxCapacity {
		panic("capacity must be between 0 and MaxCapacity")
	}
	sl := &SkipList{
		nodes:      make([]SkipListNode, capacity+1),
		count:      0,
		head:       0,
		freeHead:   1,
		level:      1,
		rd:         rand.New(rand.NewSource(seed)),
		descending: descending,
	}
	sl.head = 0
	//哨兵
	sl.nodes[0].Level = MaxLevel
	for i := 0; i < MaxLevel; i++ {
		sl.nodes[0].Forward[i] = protocol.NullIndex
	}
	//底链接
	for i := int32(1); i < capacity; i++ {
		sl.nodes[i].Forward[0] = i + 1
	}
	//last node
	sl.nodes[capacity].Forward[0] = protocol.NullIndex
	return sl
}

// insert new price
func (sl *SkipList) Insert(price udecimal.Decimal) (bool, error) {
	var update [MaxLevel]int32
	x := sl.head
	for i := MaxLevel - 1; i >= 0; i-- {
		for sl.nodes[x].Forward[i] != protocol.NullIndex && sl.less(sl.nodes[sl.nodes[x].Forward[i]].Price, price) {
			x = sl.nodes[x].Forward[i]
		}
		update[i] = x
	}
	x = sl.nodes[x].Forward[0]
	if x != protocol.NullIndex && sl.nodes[x].Price.Equal(price) {
		return false, nil
	}
	newLevel := sl.randomLevel()
	if newLevel > sl.level {
		for i := sl.level; i < newLevel; i++ {
			update[i] = sl.head
		}
		sl.level = newLevel
	}
	nodeIndex, err := sl.alloc()
	if err != nil {
		return false, err
	}
	sl.nodes[nodeIndex].Price = price
	sl.nodes[nodeIndex].Level = newLevel
	//reconnect nodes
	for i := int32(0); i < newLevel; i++ {
		sl.nodes[nodeIndex].Forward[i] = sl.nodes[update[i]].Forward[i]
		sl.nodes[update[i]].Forward[i] = nodeIndex
	}
	sl.count++
	return true, nil
}

// remove node
func (sl *SkipList) Remove(price udecimal.Decimal) (bool, error) {
	var update [MaxLevel]int32
	x := sl.head
	for i := MaxLevel - 1; i >= 0; i-- {
		for sl.nodes[x].Forward[i] != protocol.NullIndex && sl.less(sl.nodes[sl.nodes[x].Forward[i]].Price, price) {
			x = sl.nodes[x].Forward[i]
		}
		update[i] = x
	}
	x = sl.nodes[x].Forward[0]
	if x == protocol.NullIndex || !sl.nodes[x].Price.Equal(price) {
		return false, nil
	}
	//update node forward pointer
	for i := int32(0); i < sl.level; i++ {
		if sl.nodes[update[i]].Forward[i] != x {
			break
		}
		sl.nodes[update[i]].Forward[i] = sl.nodes[x].Forward[i]
	}
	//release node
	sl.freeNode(x)
	for sl.level > 1 && sl.nodes[sl.head].Forward[sl.level-1] == protocol.NullIndex {
		sl.level--
	}
	sl.count--
	return true, nil
}

// contains values
func (sl *SkipList) Contains(price udecimal.Decimal) (bool, int32) {
	x := sl.head
	for i := MaxLevel - 1; i >= 0; i-- {
		for sl.nodes[x].Forward[i] != protocol.NullIndex && sl.less(sl.nodes[sl.nodes[x].Forward[i]].Price, price) {
			x = sl.nodes[x].Forward[i]
		}
	}
	x = sl.nodes[x].Forward[0]
	if x == protocol.NullIndex {
		return false, protocol.NullIndex
	}
	return sl.nodes[x].Price.Equal(price), x
}

// min value
func (sl *SkipList) Min() (bool, udecimal.Decimal) {
	x := sl.nodes[sl.head].Forward[0]
	if x == protocol.NullIndex {
		return false, udecimal.Zero
	}
	return true, sl.nodes[x].Price
}

// remove min node
func (sl *SkipList) RemoveMin() (bool, udecimal.Decimal) {
	x := sl.nodes[sl.head].Forward[0]
	if x == protocol.NullIndex {
		return false, udecimal.Zero
	}
	minPrice := sl.nodes[x].Price
	for i := int32(0); i < sl.level; i++ {
		if sl.nodes[sl.head].Forward[i] != x {
			break
		}
		sl.nodes[sl.head].Forward[i] = sl.nodes[x].Forward[i]
	}
	sl.freeNode(x)
	for sl.level > 1 && sl.nodes[sl.head].Forward[sl.level-1] == protocol.NullIndex {
		sl.level--
	}
	sl.count--
	return true, minPrice
}

// get all values
func (sl *SkipList) GetValueSlice() []udecimal.Decimal {
	result := make([]udecimal.Decimal, 0, sl.count)
	x := sl.nodes[sl.head].Forward[0]
	for x != protocol.NullIndex {
		result = append(result, sl.nodes[x].Price)
		x = sl.nodes[x].Forward[0]
	}
	return result
}

func (sl *SkipList) LevelNodes(level int32) []udecimal.Decimal {
	if level < 0 || level > sl.level {
		return nil
	}
	x := sl.nodes[sl.head].Forward[level]
	if x == protocol.NullIndex {
		return nil
	}
	result := make([]udecimal.Decimal, 0)
	for x != protocol.NullIndex {
		result = append(result, sl.nodes[x].Price)
		x = sl.nodes[x].Forward[level]
	}
	return result
}

func (sl *SkipList) Count() int32 {
	return sl.count
}

func (sl *SkipList) Capacity() int32 {
	return int32(len(sl.nodes))
}

func (sl *SkipList) freeNode(index int32) {
	sl.nodes[index].Forward[0] = sl.freeHead
	sl.freeHead = index
}

// get slot index
func (sl *SkipList) alloc() (int32, error) {
	currentFreeHead := sl.freeHead
	if currentFreeHead == protocol.NullIndex {
		if err := sl.scale(); err != nil {
			return protocol.NullIndex, err
		}
	}
	sl.freeHead = sl.nodes[currentFreeHead].Forward[0]
	for i := 0; i < MaxLevel; i++ {
		sl.nodes[currentFreeHead].Forward[i] = protocol.NullIndex
	}
	return currentFreeHead, nil
}

// invoke scale
func (sl *SkipList) scale() error {
	oldCapacity := int32(len(sl.nodes))
	newCapacity := oldCapacity * ScaleFactor
	if newCapacity > MaxCapacity {
		if oldCapacity >= MaxCapacity {
			return errors.New("max capacity exceeded")
		}
		newCapacity = MaxCapacity
	}
	newNodes := make([]SkipListNode, newCapacity)
	copy(newNodes, sl.nodes)
	for i := oldCapacity; i < newCapacity-1; i++ {
		newNodes[i].Forward[i] = i + 1
	}
	newNodes[newCapacity-1].Forward[0] = sl.freeHead
	sl.freeHead = oldCapacity
	sl.nodes = newNodes
	return nil
}

// if a < b
func (sl *SkipList) less(a, b udecimal.Decimal) bool {
	if sl.descending {
		return a.GreaterThan(b)
	}
	return a.LessThan(b)
}
func (sl *SkipList) randomLevel() int32 {
	level := int32(1)
	// 25% change
	for level < MaxLevel && sl.rd.Intn(RandomLevelRate) == 0 {
		level++
	}
	return level
}

func (sl *SkipList) Iterator() *SkipListIterator {
	return &SkipListIterator{
		sl:      sl,
		current: sl.nodes[sl.head].Forward[0],
	}
}

func (it *SkipListIterator) Next() {
	if it.current != protocol.NullIndex {
		it.current = it.sl.nodes[it.current].Forward[0]
	}
}

func (it *SkipListIterator) Valid() bool {
	return it.current != protocol.NullIndex
}

func (it *SkipListIterator) Value() udecimal.Decimal {
	return it.sl.nodes[it.current].Price
}
