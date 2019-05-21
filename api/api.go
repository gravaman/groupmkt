package api

type Load struct {
	Key string
}

type Store struct {
	Key string
	Value string
}

type NullResult int

type ValueResult struct {
	Value string
}
