package core

import (
	"MOMEngine/protocol"
	"sync"
	"time"

	"github.com/quagmt/udecimal"
)

type OrderBookLog struct {
	SeqId        int64              `json:"seqId"`
	TradeId      int64              `json:"tradeId"`
	Type         protocol.LogType   `json:"type"`
	MarketId     string             `json:"marketId"`
	Side         protocol.Side      `json:"side"`
	Price        string             `json:"price"`  //售价
	Size         string             `json:"size"`   //数量
	Amount       string             `json:"amount"` //价格
	OrderId      string             `json:"orderId"`
	UserId       int64              `json:"userId"`
	OrderType    protocol.OrderType `json:"orderType"`
	PrePrice     string             `json:"prePrice"` //just type=amend
	PreSize      string             `json:"preSize"`
	MakerOrderId string             `json:"makerOrderId"`
	MakerUserId  int64              `json:"makerUserId"`
	RejectReason int32              `json:"rejectReason"`
	Timestamp    int64              `json:"timestamp"`
	CreateTime   time.Time          `json:"createTime"`
}

type PushLog interface {
	Publish([]*OrderBookLog)
}

type MemoryLog struct {
	mu     sync.RWMutex
	Trades []*OrderBookLog
}

var logPool = sync.Pool{
	New: func() any {
		return &OrderBookLog{}
	},
}
var logSlicePool = sync.Pool{
	New: func() any {
		temp := make([]*OrderBookLog, 0, 8)
		return &temp
	},
}

func getOrderBookLog() *OrderBookLog {
	return logPool.Get().(*OrderBookLog)
}
func releaseOrderBookLog(l *OrderBookLog) {
	*l = OrderBookLog{}
	logPool.Put(l)
}

func acquireLogSlice() *[]*OrderBookLog {
	return logSlicePool.Get().(*[]*OrderBookLog)
}
func releaseLogSlice(l *[]*OrderBookLog) {
	temp := *l
	*l = temp[:0]
	logSlicePool.Put(l)
}
func NewMemoryLog() *MemoryLog {
	return &MemoryLog{
		Trades: make([]*OrderBookLog, 0),
	}
}

// 推送日志
func (ml *MemoryLog) Publish(trades []*OrderBookLog) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	for _, trade := range trades {
		temp := &OrderBookLog{}
		*temp = *trade
		ml.Trades = append(ml.Trades, temp)
	}
}

func (ml *MemoryLog) GetLogs() []*OrderBookLog {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	logs := make([]*OrderBookLog, len(ml.Trades))
	copy(logs, ml.Trades)
	return logs
}

func NewOpenLog(seqId int64, marketId string, orderId string, userId int64, side protocol.Side, price udecimal.Decimal, size udecimal.Decimal, orderType protocol.OrderType, timestamp int64) *OrderBookLog {
	log := getOrderBookLog()
	log.SeqId = seqId
	log.Type = protocol.LogTypeOpen
	log.MarketId = marketId
	log.Side = side
	log.Price = price.String()
	log.Size = size.String()
	log.OrderId = orderId
	log.UserId = userId
	log.OrderType = orderType
	log.Timestamp = timestamp
	log.CreateTime = time.Now()
	return log
}

func NewMatchLog(seqId int64, tradeId int64, marketId string,
	orderId string, takerUserId int64, takerSide protocol.Side, takerType protocol.OrderType,
	makerId string, makerUserId int64,
	price udecimal.Decimal, size udecimal.Decimal, timestamp int64) *OrderBookLog {
	log := getOrderBookLog()
	log.SeqId = seqId
	log.Type = protocol.LogTypeMatch
	log.MarketId = marketId
	log.TradeId = tradeId
	log.Side = takerSide
	log.Size = size.String()
	log.Price = price.String()
	log.Amount = price.Mul(size).String()
	log.OrderId = orderId
	log.UserId = takerUserId
	log.OrderType = takerType
	log.MakerOrderId = makerId
	log.MakerUserId = makerUserId
	log.Timestamp = timestamp
	log.CreateTime = time.Now()
	return log
}

func NewCancelLog(seqID int64, marketID string, orderID string, userID int64, side protocol.Side, price, size udecimal.Decimal, orderType protocol.OrderType, timestamp int64) *OrderBookLog {
	log := getOrderBookLog()
	log.SeqId = seqID
	log.Type = protocol.LogTypeCancel
	log.MarketId = marketID
	log.Side = side
	log.Price = price.String()
	log.Size = size.String()
	log.OrderId = orderID
	log.UserId = userID
	log.OrderType = orderType
	log.Timestamp = timestamp
	log.CreateTime = time.Now().UTC()
	return log
}

func NewAmendLog(seqID int64, marketID string, orderID string, userID int64, side protocol.Side, price, size udecimal.Decimal, oldPrice udecimal.Decimal, oldSize udecimal.Decimal, orderType protocol.OrderType, timestamp int64) *OrderBookLog {
	log := getOrderBookLog()
	log.SeqId = seqID
	log.Type = protocol.LogTypeAmend
	log.MarketId = marketID
	log.Side = side
	log.Price = price.String()
	log.Size = size.String()
	log.PrePrice = oldPrice.String()
	log.PreSize = oldSize.String()
	log.OrderId = orderID
	log.UserId = userID
	log.OrderType = orderType
	log.Timestamp = timestamp
	log.CreateTime = time.Now().UTC()
	return log
}

func NewRejectLog(seqID int64, marketID string, orderID string, userID int64, reason int32, timestamp int64) *OrderBookLog {
	log := getOrderBookLog()
	log.SeqId = seqID
	log.Type = protocol.LogTypeReject
	log.MarketId = marketID
	log.OrderId = orderID
	log.UserId = userID
	log.RejectReason = reason
	log.Timestamp = timestamp
	log.CreateTime = time.Now().UTC()
	return log
}
