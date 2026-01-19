package protocol

import "github.com/quagmt/udecimal"

const (
	NullIndex = -1
)

type Side uint8

const (
	Buy  Side = 1
	Sell Side = 2
)

type OrderType string

const (
	TypeMarket   OrderType = "market"
	TypeLimit    OrderType = "limit"
	TypeFOK      OrderType = "fok"
	typeIOC      OrderType = "ioc"
	TypePostOnly OrderType = "postOnly"
	TypeCancel   OrderType = "cancel"
)

type Order struct {
	Id        string           `json:"id"`
	Side      Side             `json:"side"`
	Price     udecimal.Decimal `json:"price"`
	Size      udecimal.Decimal `json:"size"`
	OrderType OrderType        `json:"orderType"`
	UserId    int64            `json:"userId"`
	Timestamp int64            `json:"timestamp"`

	VisibleLimit udecimal.Decimal `json:"visibleLimit"`
	HiddenSize   udecimal.Decimal `json:"hiddenSize"`

	Prev *Order
	Next *Order
}

type OrderDepth struct {
	Price udecimal.Decimal `json:"price"`
	Size  udecimal.Decimal `json:"size"`
	Count int64
}

type Command struct {
	Version  uint8             `json:"version"`  //版本
	MarketId string            `json:"marketId"` //交易对ID
	SeqId    int64             `json:"seqId"`    //全局ID
	Type     CommandType       `json:"type"`     // 指令类型
	Payload  []byte            `json:"payload"`  //负荷
	Metadata map[string]string `json:"metadata"` //元数据
}

type CommandType uint8

const (
	CmdUnknown       CommandType = 0
	CmdCreateMarket  CommandType = 1
	CmdSuspendMarket CommandType = 2
	CmdResumeMarket  CommandType = 3
	CmdUpdateConfig  CommandType = 4

	CmdPlaceOrder  CommandType = 10
	CmdCancelOrder CommandType = 11
	CmdAmendOrder  CommandType = 12
)

type OrderBookState uint8

const (
	OrderBookRunning OrderBookState = 0 //运行中
	OrderBookPause   OrderBookState = 1 //暂停
	OrderBookStop    OrderBookState = 2 //停止
)

// 指令：挂单
type PlaceOrderCommand struct {
	OrderId      string    `json:"orderId"`
	Side         Side      `json:"side"`
	OrderType    OrderType `json:"orderType"`
	Price        string    `json:"price"`
	Size         string    `json:"size"`
	VisibleLimit string    `json:"visibleLimit"`
	QuoteSize    string    `json:"quoteSize"`
	UserId       int64     `json:"userId"`
	Timestamp    int64     `json:"timestamp"`
}

// 指令：取消订单
type CancelOrderCommand struct {
	OrderId   string `json:"orderId"`
	UserId    int64  `json:"userId"`
	Timestamp int64  `json:"timestamp"`
}

// 指令：调整挂单价格和数量
type AmendOrderCommand struct {
	OrderId   string `json:"orderId"`   //订单ID
	UserId    int64  `json:"userId"`    //用户ID
	NewPrice  string `json:"newPrice"`  //新价格
	NewSize   string `json:"newSize"`   //新数量
	Timestamp int64  `json:"timestamp"` //时间戳
}

// 指令：创建交易对
type CreateMarketCommand struct {
	UserId     int64  `json:"userId"`
	MarketId   string `json:"marketId"`
	MinLotSize string `json:"minLotSize"`
}

// 指令：暂停
type SuspendMarketCommand struct {
	UserId   int64  `json:"userId"`
	MarketId string `json:"marketId"`
	Reason   string `json:"reason"`
}

// 指令：重启
type ResumeMarketCommand struct {
	UserId   int64  `json:"userId"`
	MarketId string `json:"marketId"`
}

// 指令：更新配置
type UpdateConfigCommand struct {
	UserId     int64  `json:"userId"`
	MarketId   string `json:"marketId"`
	MinLotSize string `json:"minLotSize"`
}

// request
type RequestGetDepth struct {
	MarketId string `json:"marketId"`
	Limit    int64  `json:"limit"`
}
type RequestGetState struct {
	MarketId string `json:"marketId"`
}

type InputEvent struct {
	Cmd *Command
}

type LogType uint8

const (
	LogTypeOpen   LogType = 0
	LogTypeMatch  LogType = 1
	LogTypeCancel LogType = 2
	LogTypeAmend  LogType = 3
	LogTypeReject LogType = 4
)
