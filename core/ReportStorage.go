package core

type ReportStorage interface {
	Store(data []byte) error
}
