package core

import (
	"MOMEngine/protocol"
	"context"
	"runtime"
	"sync/atomic"
)

type RingBuffer[T any] struct {
	_           [56]byte
	producerSeq atomic.Int64
	_           [56]byte
	consumerSeq atomic.Int64
	_           [56]byte
	capacity    int64
	buffer      []T
	bufferMask  int64
	pushers     []int64
	handler     HandlerEvent[T]
	shutDown    atomic.Bool
}
type HandlerEvent[T any] interface {
	OnEvent(event *T)
}

func NewRingBuffer[T any](capacity int64, handler HandlerEvent[T]) *RingBuffer[T] {
	if capacity <= 0 || capacity%2 != 0 {
		panic("invalid capacity")
	}
	rb := &RingBuffer[T]{
		capacity:   capacity,
		buffer:     make([]T, capacity),
		bufferMask: capacity - 1,
		pushers:    make([]int64, capacity),
		handler:    handler,
	}
	rb.producerSeq.Store(protocol.NullIndex)
	rb.consumerSeq.Store(protocol.NullIndex)
	//init slot
	for i := range rb.pushers {
		atomic.StoreInt64(&rb.pushers[i], protocol.NullIndex)
	}
	return rb
}

// push event
func (rb *RingBuffer[T]) Push(e T) {
	nextSeq, slot := rb.NextSeq()
	if nextSeq == protocol.NullIndex {
		return
	}
	*slot = e
	rb.Commit(nextSeq)
}

// gen next id
func (rb *RingBuffer[T]) NextSeq() (int64, *T) {
	if rb.shutDown.Load() {
		return protocol.NullIndex, nil
	}
	var nextSeq int64
	for {
		currentProducerSeq := rb.producerSeq.Load()
		nextSeq = currentProducerSeq + 1
		wrapPoint := nextSeq - rb.capacity
		currentConsumerSeq := rb.consumerSeq.Load()
		if wrapPoint >= currentConsumerSeq {
			runtime.Gosched()
			continue
		}
		if rb.producerSeq.CompareAndSwap(currentProducerSeq, nextSeq) {
			return nextSeq, &rb.buffer[nextSeq&rb.bufferMask]
		}
		runtime.Gosched()
	}
}

// go commit
func (rb *RingBuffer[T]) Commit(seq int64) bool {
	atomic.StoreInt64(&rb.pushers[seq&rb.bufferMask], seq)
	return true
}

// go start process
func (rb *RingBuffer[T]) Start() {
	rb.shutDown.Store(false)
	go rb.consumerLoop()
}

func (rb *RingBuffer[T]) consumerLoop() {
	nextConsumerSeq := rb.consumerSeq.Load() + 1
	for {
		currentProducerSeq := rb.producerSeq.Load()
		if rb.shutDown.Load() {
			rb.processRemainEvent(nextConsumerSeq)
			return
		}
		process := false
		//batch invoke
		if nextConsumerSeq <= currentProducerSeq {
			//process index
			index := nextConsumerSeq & rb.bufferMask
			//自旋等待commit
			for atomic.LoadInt64(&rb.pushers[index]) != nextConsumerSeq {
				runtime.Gosched()
			}
			rb.handler.OnEvent(&rb.buffer[index])
			rb.consumerSeq.Store(nextConsumerSeq)
			nextConsumerSeq++
			process = true
		}
		if !process {
			runtime.Gosched()
		}
	}
}

// remain
func (rb *RingBuffer[T]) processRemainEvent(nextConsumerSeq int64) {
	currentProducerSeq := rb.producerSeq.Load()
	for nextConsumerSeq <= currentProducerSeq {
		index := nextConsumerSeq & rb.bufferMask
		for atomic.LoadInt64(&rb.pushers[index]) != nextConsumerSeq {
			runtime.Gosched()
		}
		rb.handler.OnEvent(&rb.buffer[index])
		rb.consumerSeq.Store(nextConsumerSeq)
		nextConsumerSeq++
	}
}

func (rb *RingBuffer[T]) Shutdown(ctx context.Context) error {
	rb.shutDown.Store(true)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if rb.consumerSeq.Load() >= rb.consumerSeq.Load() {
				return nil
			}
			runtime.Gosched()
		}
	}
}
func (rb *RingBuffer[T]) GetProducerSeq() int64 {
	return rb.producerSeq.Load()
}
func (rb *RingBuffer[T]) GetConsumerSeq() int64 {
	return rb.consumerSeq.Load()
}
func (rb *RingBuffer[T]) GetPendingEvents() int64 {
	producerSeq := rb.producerSeq.Load()
	consumerSeq := rb.consumerSeq.Load()
	return producerSeq - consumerSeq
}
