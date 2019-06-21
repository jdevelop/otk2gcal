package storage

type EventsStorage interface {
	AddEvents(...string) error
	IsExist(string) (bool, error)
}
