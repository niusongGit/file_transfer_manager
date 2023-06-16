/*
动态扩展的channl
*/
package utils

import (
	"context"
	"sync"
)

/*
动态大小的管道
*/
type ChanDynamic struct {
	chanLen    uint64
	lock       *sync.RWMutex
	chans      map[uint64]chan interface{}
	startIndex uint64
	endIndex   uint64
}

/*
创建一个动态管道
*/
func NewChanDynamic(chanLen uint64) *ChanDynamic {
	chans := ChanDynamic{
		lock:  new(sync.RWMutex),
		chans: make(map[uint64]chan interface{}, 0),
	}
	if chanLen == 0 {
		chans.chanLen = 10000
	} else {
		chans.chanLen = chanLen
	}
	chanOne := make(chan interface{}, chans.chanLen)
	chans.chans[chans.startIndex] = chanOne
	return &chans
}

func (this *ChanDynamic) Get(contextRoot context.Context) interface{} {
	var one interface{}
	for {
		// fmt.Println("111111111111")
		isNext := false
		this.lock.RLock()
		chanOne := this.chans[this.startIndex]
		if this.endIndex > this.startIndex {
			isNext = true
		}
		this.lock.RUnlock()
		// fmt.Println("222222222222")
		if isNext {
			have := false
			select {
			case <-contextRoot.Done():
				return nil
			case one = <-chanOne:
				have = true
			default:
			}
			// fmt.Println("333333333333")
			if have {
				// fmt.Println("4444444444444")
				return one
			} else {
				// fmt.Println("55555555555555")
				this.lock.Lock()
				delete(this.chans, this.startIndex)
				this.startIndex++
				this.lock.Unlock()
			}
		} else {
			// fmt.Println("666666666666666")
			select {
			case <-contextRoot.Done():
				return nil
			case one = <-chanOne:
				return one
			}
		}

	}

}

func (this *ChanDynamic) Add(one interface{}) {
	if one == nil {
		return
	}
	ok := false
	this.lock.RLock()
	chanOne := this.chans[this.endIndex]
	this.lock.RUnlock()

	select {
	case chanOne <- one:
		ok = true
	default:
	}
	if ok {
		return
	}

	chanOne = make(chan interface{}, this.chanLen)
	chanOne <- one
	this.lock.Lock()
	this.endIndex++
	this.chans[this.endIndex] = chanOne
	this.lock.Unlock()
}
