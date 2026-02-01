package core

import (
	"MOMEngine/protocol"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quagmt/udecimal"
)

// 订单缓存池
var orderPool = sync.Pool{
	New: func() any {
		return &protocol.Order{}
	},
}

// 下单指令池
var placeOrderCmdPool = sync.Pool{
	New: func() any {
		return &protocol.PlaceOrderCommand{}
	},
}

// 取消指令缓存
var cancelOrderCmdPool = sync.Pool{
	New: func() any {
		return &protocol.CancelOrderCommand{}
	},
}

// get order from pool
func getOrderFromPool() *protocol.Order {
	return orderPool.Get().(*protocol.Order)
}

// release order in pool
func releaseOrder(order *protocol.Order) {
	*order = protocol.Order{}
	orderPool.Put(order)
}

// 最小交易单位 0.00000001
var DefaultLotSize = udecimal.MustFromInt64(1, 8)

type OrderBook struct {
	marketId          string           //交易对ID
	lowSize           udecimal.Decimal //交易对最低交易单位
	seqId             atomic.Int64     //全局ID
	lastCmdSeqId      atomic.Int64     //最后一次处理指令ID
	tradeId           atomic.Int64     //交易ID
	shutDown          atomic.Bool
	state             protocol.OrderBookState
	bidQueue          *queue //买单队列
	askQueue          *queue //卖单队列
	cmdBuffer         *RingBuffer[protocol.InputEvent]
	done              chan struct{}
	shutdownCompleted chan struct{}
	serializer        protocol.Serializer
	traderLog         PushLog
}
type OrderBookOption func(*OrderBook)

func NewOrderBook(marketId string, tradeLog PushLog, opts ...OrderBookOption) *OrderBook {
	book := &OrderBook{
		marketId:          marketId,
		lowSize:           DefaultLotSize,
		bidQueue:          NewBuyerQueue(),
		askQueue:          NewSellerQueue(),
		done:              make(chan struct{}),
		shutdownCompleted: make(chan struct{}),
		traderLog:         tradeLog,
		serializer:        &protocol.DefaultSerializer{},
	}
	for _, opt := range opts {
		opt(book)
	}
	book.cmdBuffer = NewRingBuffer[protocol.InputEvent](60000, book)
	book.state = protocol.OrderBookRunning
	return book
}

// process event
func (b *OrderBook) OnEvent(e *protocol.InputEvent) {
	if e.Cmd != nil {
		b.processCmd(e.Cmd)
		return
	}
}

// 避免调用push拷贝一次对象
func (b *OrderBook) EnqueueCommand(cmd *protocol.Command) error {
	if b.shutDown.Load() {
		return errors.New("order book is shutting down")
	}
	seq, slot := b.cmdBuffer.NextSeq()
	if seq == protocol.NullIndex {
		return errors.New("no slot")
	}
	slot.Cmd = cmd
	b.cmdBuffer.Commit(seq)
	return nil
}

// 集中转发处理指令
func (b *OrderBook) processCmd(cmd *protocol.Command) {
	switch cmd.Type {
	case protocol.CmdSuspendMarket:
		payload := &protocol.SuspendMarketCommand{}
		if err := b.serializer.Unmarshal(cmd.Payload, &payload); err != nil {
			b.logRejectPayload("", payload.UserId, protocol.ReasonInvalidPayload, cmd.Metadata)
			return
		}
		b.handleSuspendMarket(payload)
	case protocol.CmdResumeMarket:
		payload := &protocol.ResumeMarketCommand{}
		if err := b.serializer.Unmarshal(cmd.Payload, &payload); err != nil {
			b.logRejectPayload("", payload.UserId, protocol.ReasonInvalidPayload, cmd.Metadata)
			return
		}
		b.handleResumeMarket(payload)
	case protocol.CmdPlaceOrder:
		payload := placeOrderCmdPool.Get().(*protocol.PlaceOrderCommand)
		*payload = protocol.PlaceOrderCommand{}
		if err := b.serializer.Unmarshal(cmd.Payload, &payload); err != nil {
			placeOrderCmdPool.Put(payload)
			b.logRejectPayload("", payload.UserId, protocol.ReasonInvalidPayload, cmd.Metadata)
			return
		}
		if b.state != protocol.OrderBookRunning {
			placeOrderCmdPool.Put(payload)
			b.logRejectPayload("", payload.UserId, protocol.ReasonStateHadDone, cmd.Metadata)
			return
		}
		b.handlePlaceOrder(payload)
		placeOrderCmdPool.Put(payload)
	}
}

// 下单
func (b *OrderBook) PlaceOrder(cmd *protocol.PlaceOrderCommand) error {
	if b.shutDown.Load() {
		return errors.New("order book is shutting down")
	}
	if len(cmd.OrderType) == 0 || len(cmd.OrderId) == 0 {
		return errors.New("invalid order type")
	}
	bs, err := b.serializer.Marshal(cmd)
	if err != nil {
		return err
	}
	input := &protocol.Command{
		MarketId: b.marketId,
		Type:     protocol.CmdPlaceOrder,
		Payload:  bs,
	}
	return b.EnqueueCommand(input)
}

// 修改订单
func (b *OrderBook) AmendOrder(cmd *protocol.AmendOrderCommand) error {
	if b.shutDown.Load() {
		return errors.New("order book is shutting down")
	}
	if len(cmd.OrderId) == 0 {
		return errors.New("invalid order id")
	}
	bs, err := b.serializer.Marshal(cmd)
	if err != nil {
		return err
	}
	input := &protocol.Command{
		MarketId: b.marketId,
		Type:     protocol.CmdAmendOrder,
		Payload:  bs,
	}
	return b.EnqueueCommand(input)
}

// 撤销订单
func (b *OrderBook) CancelOrder(cmd *protocol.CancelOrderCommand) error {
	if b.shutDown.Load() {
		return errors.New("order book is shutting down")
	}
	if len(cmd.OrderId) == 0 {
		return errors.New("invalid order id")
	}
	bs, err := b.serializer.Marshal(cmd)
	if err != nil {
		return err
	}
	input := &protocol.Command{
		MarketId: b.marketId,
		Type:     protocol.CmdCancelOrder,
		Payload:  bs,
	}
	return b.EnqueueCommand(input)
}
func (b *OrderBook) logRejectPayload(orderId string, userId int64, reasonCode int32, _ map[string]string) {
	logs := acquireLogSlice()
	log := NewRejectLog(b.seqId.Add(1), b.marketId, orderId, userId, reasonCode, time.Now().Unix())
	*logs = append(*logs, log)
	b.traderLog.Publish(*logs)
	releaseOrderBookLog(log)
	releaseLogSlice(logs)
}

// 暂停
func (b *OrderBook) handleSuspendMarket(bean *protocol.SuspendMarketCommand) {
	if b.state == protocol.OrderBookStop {
		b.logRejectPayload("", bean.UserId, protocol.ReasonStateHadDone, nil)
		return
	}
	b.state = protocol.OrderBookPause
}

// 恢复
func (b *OrderBook) handleResumeMarket(bean *protocol.ResumeMarketCommand) {
	if b.state == protocol.OrderBookStop {
		b.logRejectPayload("", bean.UserId, protocol.ReasonStateHadDone, nil)
		return
	}
	b.state = protocol.OrderBookRunning
}

// 处理下单指令
func (b *OrderBook) handlePlaceOrder(bean *protocol.PlaceOrderCommand) {
	price, err := udecimal.Parse(bean.Price)
	if err != nil {
		b.logRejectPayload(bean.OrderId, bean.UserId, protocol.ReasonInvalidPayload, nil)
		return
	}
	size, err := udecimal.Parse(bean.Size)
	if err != nil {
		b.logRejectPayload(bean.OrderId, bean.UserId, protocol.ReasonInvalidPayload, nil)
		return
	}
	visibleLimit, _ := udecimal.Parse(bean.VisibleLimit)
	quoteSize, _ := udecimal.Parse(bean.QuoteSize)
	//repeat orde
	if b.bidQueue.GetOrder(bean.OrderId) != nil || b.askQueue.GetOrder(bean.OrderId) != nil {
		b.logRejectPayload(bean.OrderId, bean.UserId, protocol.ReasonDuplicateOrderID, nil)
		return
	}
	order := getOrderFromPool()
	order.Id = bean.OrderId
	order.Side = bean.Side
	order.Price = price
	order.Size = size
	order.OrderType = bean.OrderType
	order.UserId = bean.UserId
	order.Timestamp = bean.Timestamp
	if visibleLimit.GreaterThan(udecimal.Zero) && visibleLimit.LessThan(size) {
		order.VisibleLimit = visibleLimit
	}
	switch order.OrderType {
	case protocol.TypeMarket:
		b.processMarketOrder(order, quoteSize)
	case protocol.TypeLimit:
		b.processLimitOrder(order)
	default:

	}
	releaseOrder(order)
}

// 处理市价单，按数量或按金额
func (b *OrderBook) processMarketOrder(order *protocol.Order, quoteSize udecimal.Decimal) *[]*OrderBookLog {
	var targetQueue *queue
	if order.Side == protocol.Buy {
		targetQueue = b.askQueue
	} else {
		targetQueue = b.bidQueue
	}
	logs := acquireLogSlice()
	for {
		tempOrder := targetQueue.PeakHeadOrder()
		if tempOrder == nil {
			//havent order
			log := NewRejectLog(b.seqId.Add(1), b.marketId, order.Id, order.UserId, protocol.ReasonNoLiquidity, order.Timestamp)
			log.Side = order.Side
			if order.OrderType == protocol.TypeMarket && !quoteSize.IsZero() {
				log.Size = quoteSize.String()
			} else {
				log.Size = order.Size.String()
			}
			log.Price = order.Price.String()
			log.OrderType = order.OrderType
			*logs = append(*logs, log)
			break
		}
		matchSize := order.Size
		useQuote := matchSize.IsZero() && order.OrderType == protocol.TypeMarket && !quoteSize.IsZero()
		if useQuote {
			//按金额 根据对手盘计算能换多少量 金额/价格
			matchSize, _ = quoteSize.Div(tempOrder.Price)
		}
		if matchSize.GreaterThan(tempOrder.Size) {
			matchSize = tempOrder.Size
		}
		//check low size
		if matchSize.LessThan(b.lowSize) {
			log := NewRejectLog(b.seqId.Add(1), b.marketId, order.Id, order.UserId, protocol.ReasonLowSize, order.Timestamp)
			log.Side = order.Side
			if order.OrderType == protocol.TypeMarket && !quoteSize.IsZero() {
				log.Size = quoteSize.String()
			} else {
				log.Size = order.Size.String()
			}
			log.Price = order.Price.String()
			log.OrderType = order.OrderType
			*logs = append(*logs, log)
			break
		}
		log := NewMatchLog(b.seqId.Add(1), b.tradeId.Add(1), b.marketId, order.Id, order.UserId, order.Side, order.OrderType, tempOrder.Id, tempOrder.UserId, tempOrder.Price, matchSize, order.Timestamp)
		*logs = append(*logs, log)
		if useQuote {
			quoteSize = quoteSize.Sub(matchSize.Mul(tempOrder.Price))
		} else {
			order.Size = order.Size.Sub(matchSize)
		}
		tempOrder = targetQueue.PopHeadOrder()
		if matchSize.Equal(tempOrder.Size) {
			//完全成交，冰山单？
			b.checkIcebergOrder(order, targetQueue, logs)
		} else {
			tempOrder.Size = tempOrder.Size.Sub(matchSize)
			targetQueue.PutOrder(tempOrder, true)
		}
		if useQuote && quoteSize.IsZero() || (!useQuote && order.Size.IsZero()) {
			break
		}
	}
	return logs
}

// 处理限价单，满足条件就吃，否则直接挂单
func (b *OrderBook) processLimitOrder(order *protocol.Order) *[]*OrderBookLog {
	var orderQueue, targetQueue *queue
	if order.Side == protocol.Buy {
		targetQueue = b.askQueue
		orderQueue = b.bidQueue
	} else {
		targetQueue = b.bidQueue
		orderQueue = b.askQueue
	}
	logs := acquireLogSlice()
	for {
		tempOrder := targetQueue.PeakHeadOrder()
		if tempOrder == nil {
			//no target, put to order queue
			orderQueue.PutOrder(order, false)
			log := NewOpenLog(b.seqId.Add(1), b.marketId, order.Id, order.UserId, order.Side, order.Price, order.Size, order.OrderType, order.Timestamp)
			*logs = append(*logs, log)
			break
		}
		if (order.Side == protocol.Buy && order.Price.LessThan(tempOrder.Price)) ||
			(order.Side == protocol.Sell && order.Price.GreaterThan(tempOrder.Price)) {
			orderQueue.PutOrder(order, false)
			log := NewOpenLog(b.seqId.Add(1), b.marketId, order.Id, order.UserId, order.Side, order.Price, order.Size, order.OrderType, order.Timestamp)
			*logs = append(*logs, log)
			break
		}
		tempOrder = targetQueue.PopHeadOrder()
		if order.Size.GreaterThanOrEqual(tempOrder.Size) {
			//足够
			log := NewMatchLog(b.seqId.Add(1), b.tradeId.Add(1), b.marketId, order.Id, order.UserId, order.Side, order.OrderType, tempOrder.Id, tempOrder.UserId, tempOrder.Price, tempOrder.Size, order.Timestamp)
			*logs = append(*logs, log)
			order.Size = order.Size.Sub(tempOrder.Size)
			if !b.checkIcebergOrder(tempOrder, targetQueue, logs) {
				releaseOrder(tempOrder)
			}
			if order.Size.Equal(udecimal.Zero) {
				break
			}
		} else {
			//not enough
			log := NewMatchLog(b.seqId.Add(1), b.tradeId.Add(1), b.marketId, order.Id, order.UserId, order.Side, order.OrderType, tempOrder.Id, tempOrder.UserId, tempOrder.Price, order.Size, order.Timestamp)
			*logs = append(*logs, log)
			tempOrder.Size = tempOrder.Size.Sub(order.Size)
			targetQueue.PutOrder(tempOrder, true)
			break
		}
	}
	return logs
}

// 检查冰山订单，如果是冰山订单补货后加入队列尾部
func (b *OrderBook) checkIcebergOrder(order *protocol.Order, queue *queue, logs *[]*OrderBookLog) bool {
	if order.HiddenSize.GreaterThan(udecimal.Zero) {
		limit := order.VisibleLimit
		if order.HiddenSize.LessThan(limit) {
			limit = order.HiddenSize
		}
		order.Size = limit
		order.HiddenSize = order.HiddenSize.Sub(limit)
		queue.PutOrder(order, false)

		log := NewOpenLog(b.seqId.Add(1), b.marketId, order.Id, order.UserId, order.Side, order.Price, order.Size, order.OrderType, order.Timestamp)
		*logs = append(*logs, log)
		return true
	}
	return false
}
