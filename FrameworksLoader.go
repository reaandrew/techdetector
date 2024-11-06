package main

type FrameworksLoader interface {
	LoadAllFrameworks() ([]Framework, error)
}
