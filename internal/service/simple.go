package service

type Simple struct{}

func NewSimple() *Simple {
	return &Simple{}
}

func (simple *Simple) Get() string {
	return "22"
}
