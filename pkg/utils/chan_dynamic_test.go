package utils

import (
	"context"
	"testing"
	"time"
)

func TestChanDynamic(t *testing.T) {
	// 初始化ChanDynamic
	cd := NewChanDynamic(2)
	// 向ChanDynamic添加元素
	cd.Add("a")
	cd.Add("b")
	cd.Add("c")
	// 获取元素并判断顺序
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	val1 := cd.Get(ctx)
	val2 := cd.Get(ctx)
	val3 := cd.Get(ctx)
	if val1 != "a" || val2 != "b" || val3 != "c" {
		t.Errorf("unexpected result: %v, %v, %v", val1, val2, val3)
	}
	// 测试context超时
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if val := cd.Get(ctx); val != nil {
		t.Errorf("unexpected result: %v", val)
	}
}
