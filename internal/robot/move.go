package robot

import (
	"context"
	"math"
	"time"

	"github.com/Drunk6904/mqbot/protocol"
)

func MoveTo(ctx context.Context, s *protocol.StatusBody, x float64, y float64) {
	// 记录上一次更新的时间
	lastUpdate := time.Now()
	var reached bool

	for !reached {
		select {
		case <-ctx.Done():
			return
		default:
			// 计算当前时间
			now := time.Now()
			// 计算时间间隔（秒）
			elapsed := now.Sub(lastUpdate).Seconds()
			lastUpdate = now

			// 计算目标方向
			dx := x - s.X
			dy := y - s.Y
			distance := math.Sqrt(dx*dx + dy*dy)

			// 如果距离太近，不需要移动
			if distance < 0.1 {
				reached = true
				continue
			}

			// 计算移动方向
			direction := math.Atan2(dy, dx)

			// 计算应该移动的距离（速度 * 时间）
			moveDistance := s.Speed * elapsed

			// 确保不会超过目标距离
			if moveDistance > distance {
				moveDistance = distance * 0.95 // 到达目标前就减速
			}

			// 计算新位置
			newX := s.X + moveDistance*math.Cos(direction)
			newY := s.Y + moveDistance*math.Sin(direction)

			// 更新位置
			s.X = newX
			s.Y = newY

			// 使用固定的时间间隔来更新位置
			time.Sleep(50 * time.Millisecond)
		}
	}
}
