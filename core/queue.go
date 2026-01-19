package core

import (
	"MOMEngine/protocol"
	"errors"
	"github.com/quagmt/udecimal"
	"time"
)

type UpdateEvent struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type priceUnit struct {
	totalSize udecimal.Decimal
	head      *protocol.Order
	tail      *protocol.Order
	count     int64
}

// 双向订单链表
type queue struct {
	size        protocol.Side
	totalOrders int64
	depths      int64
	orderPrices *SkipList
	priceList   map[udecimal.Decimal]*priceUnit
	orders      map[string]*protocol.Order
}

const PriceCapacity = 102400

func NewBuyerQueue() *queue {
	return &queue{
		size:        protocol.Buy,
		orderPrices: NewSkipList(PriceCapacity, time.Now().Unix(), true),
		priceList:   make(map[udecimal.Decimal]*priceUnit),
		orders:      make(map[string]*protocol.Order),
	}
}

func NewSellerQueue() *queue {
	return &queue{
		size:        protocol.Sell,
		orderPrices: NewSkipList(PriceCapacity, time.Now().Unix(), false),
		priceList:   make(map[udecimal.Decimal]*priceUnit),
		orders:      make(map[string]*protocol.Order),
	}
}

func (q *queue) GetOrder(id string) *protocol.Order {
	return q.orders[id]
}

// 添加订单
func (q *queue) PutOrder(order *protocol.Order, isFront bool) error {
	if order == nil || len(order.Id) <= 0 || order.Price.LessThanOrEqual(udecimal.Zero) {
		return errors.New("Put New Order failed!")
	}
	unit, ok := q.priceList[order.Price]
	if !ok {
		//no price orders, init first one
		unit = &priceUnit{
			head:      order,
			tail:      order,
			totalSize: order.Size,
			count:     1,
		}
		order.Prev = nil
		order.Next = nil

		q.orders[order.Id] = order
		q.priceList[order.Price] = unit
		q.orderPrices.Insert(order.Price)
		q.totalOrders++
		q.depths++
	} else {
		if isFront {
			//put first position
			order.Next = unit.head
			order.Prev = nil
			if unit.head != nil {
				unit.head.Prev = order
			}
			unit.head = order
		} else {
			order.Prev = unit.tail
			order.Next = nil
			if unit.tail != nil {
				unit.tail.Next = order
			}
			unit.tail = order
			if unit.head == nil {
				unit.head = order
			}
		}
		unit.totalSize = unit.totalSize.Add(order.Size)
		unit.count++
		q.orders[order.Id] = order
		q.totalOrders++
	}
	return nil
}

// 移除订单
func (q *queue) RemoveOrder(id string, price udecimal.Decimal) (bool, error) {
	unit, ok := q.priceList[price]
	if !ok {
		return false, errors.New("not found price of order")
	}
	order, ok := q.orders[id]
	if !ok {
		return false, errors.New("not found order by id")
	}
	if order.Prev != nil {
		order.Prev.Next = order.Next
	} else {
		unit.head = order.Next
	}
	if order.Next != nil {
		order.Next.Prev = order.Prev
	} else {
		unit.tail = order.Prev
	}
	order.Next = nil
	order.Prev = nil

	unit.totalSize = unit.totalSize.Sub(order.Size)
	unit.count--
	//remove from order map
	delete(q.orders, id)
	q.totalOrders--
	//if current price level no order,remove
	if unit.count <= 0 {
		q.orderPrices.Remove(price)
		delete(q.priceList, price)
		q.depths--
	}
	return true, nil
}

// update order size, if newSize less or equal zero,it will bi remove
func (q *queue) UpdateOrderSize(id string, newSize udecimal.Decimal) error {
	order, ok := q.orders[id]
	if !ok {
		return errors.New("not found order by id")
	}
	unit, ok := q.priceList[order.Price]
	if !ok {
		return errors.New("not found price of order")
	}
	if newSize.LessThanOrEqual(udecimal.Zero) {
		_, e := q.RemoveOrder(id, order.Price)
		return e
	}
	diff := order.Size.Sub(newSize)
	unit.totalSize = unit.totalSize.Sub(diff)
	order.Size = newSize
	return nil
}

// get first best price
func (q *queue) PeakHeadOrder() *protocol.Order {
	ok, price := q.orderPrices.Min()
	if !ok {
		return nil
	}
	unit, ok := q.priceList[price]
	if !ok {
		return nil
	}
	return unit.head
}

// get and remove head order
func (q *queue) PopHeadOrder() *protocol.Order {
	order := q.PeakHeadOrder()
	if order == nil {
		return nil
	}
	q.RemoveOrder(order.Id, order.Price)
	return order
}
func (q *queue) OrderCount() int64 {
	return q.totalOrders
}
func (q *queue) OrderDepth() int64 {
	return q.depths
}

// 获取快照
func (q *queue) GetSnapshot() []*protocol.Order {
	snapshots := make([]*protocol.Order, 0, q.totalOrders)
	it := q.orderPrices.Iterator()
	for it.Valid() {
		price := it.Value()
		unit, ok := q.priceList[price]
		if !ok {
			it.Next()
			continue
		}
		order := unit.head
		for order != nil {
			od := &protocol.Order{
				Id:           order.Id,
				Side:         order.Side,
				Price:        order.Price,
				Size:         order.Size,
				UserId:       order.UserId,
				OrderType:    order.OrderType,
				Timestamp:    order.Timestamp,
				VisibleLimit: order.VisibleLimit,
				HiddenSize:   order.HiddenSize,
			}
			snapshots = append(snapshots, od)
			order = order.Next
		}
		it.Next()
	}
	return snapshots
}

func (q *queue) GetDepth(limit int32) []*protocol.OrderDepth {
	result := make([]*protocol.OrderDepth, 0, limit)
	count := int32(0)
	it := q.orderPrices.Iterator()
	for it.Valid() && count < limit {
		price := it.Value()
		priceUnit, ok := q.priceList[price]
		if ok {
			dd := &protocol.OrderDepth{
				Price: price,
				Size:  priceUnit.totalSize,
				Count: priceUnit.count,
			}
			result = append(result, dd)
			count++
		}
		it.Next()
	}
	return result
}
