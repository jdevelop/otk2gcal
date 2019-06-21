package storage

type noop struct{}

var noopV = &noop{}

func NewNoop() *noop {
	return noopV
}

func (n *noop) AddEvents(...string) error {
	return nil
}

func (n *noop) IsExist(string) (bool, error) {
	return false, nil
}

var _ EventsStorage = noopV
