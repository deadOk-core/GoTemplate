package positions

import "context"

func (p *Pool) RunOnTicker(scanCtx, workerCtx context.Context, clock ticker) {
	p.run(scanCtx, workerCtx, clock)
}
