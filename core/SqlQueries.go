package core

type SqlQuery struct {
	Name  string `yaml:"name"`
	Query string `yaml:"query"`
}

// SqlQueries holds a collection of SqlQuery instances.
type SqlQueries struct {
	Queries []SqlQuery `yaml:"queries"`
}
