package terminal

import "time"

const (
	outputCoalesceBytes = 8 * 1024
	outputCoalesceDelay = 5 * time.Millisecond
	outputEchoBypass    = 1024
)

type outputCoalescer struct {
	buffer []byte
	timer  *time.Timer
	ready  <-chan time.Time
	flush  func([]byte)
}

func newOutputCoalescer(flush func([]byte)) *outputCoalescer {
	return &outputCoalescer{buffer: make([]byte, 0, outputCoalesceBytes), flush: flush}
}

func (c *outputCoalescer) Push(input []byte) {
	if len(input) == 0 {
		return
	}
	if len(c.buffer) == 0 && len(input) < outputEchoBypass {
		c.flush(input)
		return
	}
	c.buffer = append(c.buffer, input...)
	if len(c.buffer) >= outputCoalesceBytes {
		c.Flush()
		return
	}
	c.arm()
}

func (c *outputCoalescer) Ready() <-chan time.Time { return c.ready }

func (c *outputCoalescer) Flush() {
	if c.timer != nil {
		if !c.timer.Stop() {
			select {
			case <-c.timer.C:
			default:
			}
		}
		c.ready = nil
	}
	if len(c.buffer) == 0 {
		return
	}
	output := append([]byte(nil), c.buffer...)
	c.buffer = c.buffer[:0]
	c.flush(output)
}

func (c *outputCoalescer) arm() {
	if c.timer == nil {
		c.timer = time.NewTimer(outputCoalesceDelay)
	} else {
		if !c.timer.Stop() {
			select {
			case <-c.timer.C:
			default:
			}
		}
		c.timer.Reset(outputCoalesceDelay)
	}
	c.ready = c.timer.C
}
