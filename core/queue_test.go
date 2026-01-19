package core

import (
	"MOMEngine/protocol"
	"testing"

	"github.com/quagmt/udecimal"
)

// 辅助断言函数
func assertInt64(t *testing.T, expected, actual int64, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %d, got %d", msg, expected, actual)
	}
}

func assertString(t *testing.T, expected, actual string, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %s, got %s", msg, expected, actual)
	}
}

func assertDecimal(t *testing.T, expected, actual udecimal.Decimal, msg string) {
	if !expected.Equal(actual) {
		t.Errorf("%s: expected %s, got %s", msg, expected, actual)
	}
}

func assertBool(t *testing.T, expected, actual bool, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

func assertError(t *testing.T, err error, wantErr bool, msg string) {
	if wantErr && err == nil {
		t.Errorf("%s: expected error, got nil", msg)
	}
	if !wantErr && err != nil {
		t.Errorf("%s: expected no error, got %v", msg, err)
	}
}

// 测试买单队列基本操作
func TestBuyerQueue_Basic(t *testing.T) {
	q := NewBuyerQueue()

	// 1. 插入订单
	// 价格 50, 数量 10
	o1 := &protocol.Order{
		Id:    "100",
		Price: udecimal.MustFromInt64(50, 0),
		Size:  udecimal.MustFromInt64(10, 0),
	}
	err := q.PutOrder(o1, false)
	assertError(t, err, false, "PutOrder o1")

	// 价格 50, 数量 5 (同价格)
	o2 := &protocol.Order{
		Id:    "101",
		Price: udecimal.MustFromInt64(50, 0),
		Size:  udecimal.MustFromInt64(5, 0),
	}
	err = q.PutOrder(o2, false)
	assertError(t, err, false, "PutOrder o2")

	// 价格 60, 数量 2 (更高价格)
	o3 := &protocol.Order{
		Id:    "102",
		Price: udecimal.MustFromInt64(60, 0),
		Size:  udecimal.MustFromInt64(2, 0),
	}
	err = q.PutOrder(o3, false)
	assertError(t, err, false, "PutOrder o3")

	// 2. 验证状态
	assertInt64(t, 3, q.OrderCount(), "OrderCount")
	assertInt64(t, 2, q.OrderDepth(), "OrderDepth") // 50 和 60 两个价格档位

	// 3. 验证最优价格 (买单应该是最高价 60)
	head := q.PeakHeadOrder()
	if head == nil {
		t.Fatal("Head order is nil")
	}
	assertString(t, "102", head.Id, "Head order ID")
	assertDecimal(t, udecimal.MustFromInt64(60, 0), head.Price, "Head order Price")

	// 4. 验证深度列表顺序 (应该是 60, 50)
	depths := q.GetDepth(10)
	if len(depths) != 2 {
		t.Fatalf("Expected depth len 2, got %d", len(depths))
	}
	// 第一档 60
	assertDecimal(t, udecimal.MustFromInt64(60, 0), depths[0].Price, "Depth[0] Price")
	assertDecimal(t, udecimal.MustFromInt64(2, 0), depths[0].Size, "Depth[0] Size")
	// 第二档 50 (总数量 10+5=15)
	assertDecimal(t, udecimal.MustFromInt64(50, 0), depths[1].Price, "Depth[1] Price")
	assertDecimal(t, udecimal.MustFromInt64(15, 0), depths[1].Size, "Depth[1] Size")
	assertInt64(t, 2, depths[1].Count, "Depth[1] Count")
}

// 测试卖单队列基本操作
func TestSellerQueue_Basic(t *testing.T) {
	q := NewSellerQueue()

	// 1. 插入订单
	// 价格 50, 数量 10
	o1 := &protocol.Order{
		Id:    "200",
		Price: udecimal.MustFromInt64(50, 0),
		Size:  udecimal.MustFromInt64(10, 0),
	}
	q.PutOrder(o1, false)

	// 价格 40, 数量 5 (更低价格)
	o2 := &protocol.Order{
		Id:    "201",
		Price: udecimal.MustFromInt64(40, 0),
		Size:  udecimal.MustFromInt64(5, 0),
	}
	q.PutOrder(o2, false)

	// 2. 验证最优价格 (卖单应该是最低价 40)
	head := q.PeakHeadOrder()
	if head == nil {
		t.Fatal("Head order is nil")
	}
	assertString(t, "201", head.Id, "Head order ID")
	assertDecimal(t, udecimal.MustFromInt64(40, 0), head.Price, "Head order Price")

	// 3. 验证深度列表顺序 (应该是 40, 50)
	depths := q.GetDepth(10)
	assertDecimal(t, udecimal.MustFromInt64(40, 0), depths[0].Price, "Depth[0] Price")
	assertDecimal(t, udecimal.MustFromInt64(50, 0), depths[1].Price, "Depth[1] Price")
}

// 测试订单移除
func TestQueue_RemoveOrder(t *testing.T) {
	q := NewBuyerQueue()
	price := udecimal.MustFromInt64(100, 0)

	o1 := &protocol.Order{Id: "1", Price: price, Size: udecimal.MustFromInt64(1, 0)}
	o2 := &protocol.Order{Id: "2", Price: price, Size: udecimal.MustFromInt64(1, 0)}
	o3 := &protocol.Order{Id: "3", Price: price, Size: udecimal.MustFromInt64(1, 0)}

	q.PutOrder(o1, false)
	q.PutOrder(o2, false)
	q.PutOrder(o3, false)

	assertInt64(t, 3, q.OrderCount(), "Initial count")

	// 1. 移除中间订单
	ok, err := q.RemoveOrder("2", price)
	assertBool(t, true, ok, "Remove middle order")
	assertError(t, err, false, "Remove middle order error")
	assertInt64(t, 2, q.OrderCount(), "Count after remove")

	// 验证链表连接: 1 -> 3
	head := q.PeakHeadOrder() // 应该是 1
	assertString(t, "1", head.Id, "Head ID")
	assertString(t, "3", head.Next.Id, "Head Next ID")

	// 2. 移除头部订单
	ok, err = q.RemoveOrder("1", price)
	assertBool(t, true, ok, "Remove head order")

	head = q.PeakHeadOrder() // 应该是 3
	assertString(t, "3", head.Id, "New Head ID")

	// 3. 移除最后一个订单
	ok, err = q.RemoveOrder("3", price)
	assertBool(t, true, ok, "Remove last order")

	assertInt64(t, 0, q.OrderCount(), "Count should be 0")
	assertInt64(t, 0, q.OrderDepth(), "Depth should be 0")
	if q.PeakHeadOrder() != nil {
		t.Error("Head should be nil")
	}

	// 4. 移除不存在的订单
	ok, err = q.RemoveOrder("999", price)
	assertBool(t, false, ok, "Remove non-existent order")
	assertError(t, err, true, "Remove non-existent order error")
}

// 测试更新订单数量
func TestQueue_UpdateOrderSize(t *testing.T) {
	q := NewBuyerQueue()
	price := udecimal.MustFromInt64(100, 0)
	o1 := &protocol.Order{Id: "1", Price: price, Size: udecimal.MustFromInt64(10, 0)}
	q.PutOrder(o1, false)

	// 1. 正常更新减少
	newSize := udecimal.MustFromInt64(5, 0)
	err := q.UpdateOrderSize("1", newSize)
	assertError(t, err, false, "Update size")

	// 验证订单大小和深度总大小
	assertDecimal(t, newSize, q.GetOrder("1").Size, "Order Size")
	depths := q.GetDepth(1)
	assertDecimal(t, newSize, depths[0].Size, "Depth Size")

	// 2. 更新到 0 (应该移除)
	err = q.UpdateOrderSize("1", udecimal.Zero)
	assertError(t, err, false, "Update size to 0")
	assertInt64(t, 0, q.OrderCount(), "Order should be removed")
}

// 测试 PopHeadOrder
func TestQueue_PopHeadOrder(t *testing.T) {
	q := NewBuyerQueue()
	q.PutOrder(&protocol.Order{Id: "1", Price: udecimal.MustFromInt64(10, 0), Size: udecimal.One}, false)
	q.PutOrder(&protocol.Order{Id: "2", Price: udecimal.MustFromInt64(20, 0), Size: udecimal.One}, false)

	// 第一次 Pop (最高价 20)
	o := q.PopHeadOrder()
	assertString(t, "2", o.Id, "First pop")
	assertInt64(t, 1, q.OrderCount(), "Count after pop")

	// 第二次 Pop (10)
	o = q.PopHeadOrder()
	assertString(t, "1", o.Id, "Second pop")
	assertInt64(t, 0, q.OrderCount(), "Count after second pop")

	// 第三次 Pop (空)
	o = q.PopHeadOrder()
	if o != nil {
		t.Error("Pop from empty queue should return nil")
	}
}

// 测试 PutOrder 的 isFront 参数
func TestQueue_PutOrder_Front(t *testing.T) {
	q := NewBuyerQueue()
	price := udecimal.MustFromInt64(100, 0)

	// 插入第一个
	q.PutOrder(&protocol.Order{Id: "1", Price: price, Size: udecimal.One}, false)

	// 正常插入 (后端)
	q.PutOrder(&protocol.Order{Id: "2", Price: price, Size: udecimal.One}, false)

	// 前端插入
	q.PutOrder(&protocol.Order{Id: "3", Price: price, Size: udecimal.One}, true)

	// 顺序应该是: 3 -> 1 -> 2
	head := q.PeakHeadOrder()
	assertString(t, "3", head.Id, "Head should be 3")
	assertString(t, "1", head.Next.Id, "Second should be 1")
	assertString(t, "2", head.Next.Next.Id, "Third should be 2")
}

// 测试获取快照
func TestQueue_GetSnapshot(t *testing.T) {
	q := NewBuyerQueue()
	o1 := &protocol.Order{Id: "1", Price: udecimal.MustFromInt64(50, 0), Size: udecimal.One}
	o2 := &protocol.Order{Id: "2", Price: udecimal.MustFromInt64(60, 0), Size: udecimal.One}

	q.PutOrder(o1, false)
	q.PutOrder(o2, false)

	snapshot := q.GetSnapshot()
	if len(snapshot) != 2 {
		t.Errorf("Expected snapshot len 2, got %d", len(snapshot))
	}

	// 快照顺序应该是按价格排序 (60 -> 50)
	assertString(t, "2", snapshot[0].Id, "First snapshot item")
	assertString(t, "1", snapshot[1].Id, "Second snapshot item")
}
