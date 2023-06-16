package utils

import (
	"context"
	"testing"
	"time"
)

func TestBackoffTimer(t *testing.T) {
	bt := NewBackoffTimer(1, 2, 4, 8)
	// 测试正常情况
	for i := 0; i < 4; i++ {
		n := bt.Wait()
		if n != int64(1<<i) {
			t.Errorf("返回的时间间隔不符合预期: %d", n)
		}
	}
	// 测试异常情况
	bt.Release()
	n := bt.Wait()
	if n != int64(8) {
		t.Errorf("释放后返回的时间间隔不符合预期: %d", n)
	}
}
func TestBackoffTimerChan(t *testing.T) {
	btc := NewBackoffTimerChan(100*time.Millisecond, 200*time.Millisecond, 400*time.Millisecond)
	// 测试正常情况
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	for i := 0; i < 3; i++ {
		d := btc.Wait(ctx)
		if d != btc.interval[i] {
			t.Errorf("返回的时间间隔不符合预期: %v", d)
		}
	}
	// 测试异常情况
	btc.Reset()
	btc.Release()
	d := btc.Wait(ctx)
	if d != 0 {
		t.Errorf("重置和释放后返回的时间间隔不符合预期: %v", d)
	}
}
