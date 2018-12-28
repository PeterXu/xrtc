package util

type GoPool struct {
	work chan func()
	sema chan struct{}
}

func NewGoPool(size int) *GoPool {
	return &GoPool{
		work: make(chan func()),
		sema: make(chan struct{}, size),
	}
}

func (p *GoPool) Schedule(task func()) error {
	select {
	case p.work <- task:
	case p.sema <- struct{}{}:
		go p.worker(task)
	}
	return nil
}

func (p *GoPool) worker(task func()) {
	defer func() { <-p.sema }()
	for {
		task()
		task = <-p.work
	}
}
