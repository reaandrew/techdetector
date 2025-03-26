package core

type PostScanner interface {
	Scan(path, name string) ([]Finding, error)
}
