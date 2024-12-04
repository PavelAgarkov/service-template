package pkg

type Repository interface {
	Shutdown() func()
}
