package core

import (
	"MOMEngine/protocol"
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
func getOrder() *protocol.Order {
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
	lotSize           udecimal.Decimal //交易对最低交易单位
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
		lotSize:           DefaultLotSize,
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

func (b *OrderBook) processCmd(cmd *protocol.Command) {
	switch cmd.Type {
	case protocol.CmdSuspendMarket:
		payload := &protocol.SuspendMarketCommand{}
		if err := b.serializer.Unmarshal(cmd.Payload, &payload); err != nil {
			b.logRejectPayload("", payload.UserId, protocol.ReasonInvalidPayload, cmd.Metadata)
			return
		}
	}
}

func (b *OrderBook) logRejectPayload(orderId string, userId int64, reasonCode int32, _ map[string]string) {
	logs := acquireLogSlice()
	log := NewRejectLog(b.seqId.Add(1), b.marketId, orderId, userId, reasonCode, time.Now().Unix())
	*logs = append(*logs, log)
	b.traderLog.Publish(*logs)
	releaseOrderBookLog(log)
	releaseLogSlice(logs)
}

// updates the order book state to Suspended.
func (b *OrderBook) handleSuspendMarket(bean *protocol.SuspendMarketCommand) {
	if b.state == protocol.OrderBookStop {
		b.logRejectPayload("", bean.UserId, protocol.ReasonStateHadDone, nil)
		return
	}
	b.state = protocol.OrderBookPause
}
