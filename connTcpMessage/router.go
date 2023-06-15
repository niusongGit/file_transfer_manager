package connTcpMessage

import "sync"

type Router struct {
	handlersMapping map[uint64]MessageHandler
	lock            *sync.RWMutex
}

func (r *Router) AddRouter(mid uint64, handler MessageHandler) {
	if mid < 1 {
		panic("mid 必须大于或等于1！！")
	}
	r.lock.Lock()
	r.handlersMapping[mid] = handler
	r.lock.Unlock()
}

func (r *Router) GetRouter(mid uint64) MessageHandler {
	r.lock.RLock()
	h := r.handlersMapping[mid]
	r.lock.RUnlock()
	return h
}

func NewRouter() *Router {
	return &Router{
		handlersMapping: make(map[uint64]MessageHandler),
		lock:            new(sync.RWMutex),
	}
}
