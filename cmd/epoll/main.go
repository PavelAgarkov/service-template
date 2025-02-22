package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

func main() {
	epfd, _ := unix.EpollCreate1(0)
	defer unix.Close(epfd)

	stdinFd := int(os.Stdin.Fd())
	event := unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(stdinFd)}
	unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, stdinFd, &event)

	events := make([]unix.EpollEvent, 10)

	fmt.Println("Waiting for input...")

	for {
		n, _ := unix.EpollWait(epfd, events, -1)
		for i := 0; i < n; i++ {
			if events[i].Fd == int32(stdinFd) {
				buf := make([]byte, 1024)
				count, _ := os.Stdin.Read(buf)

				// Пишем напрямую в os.Stdout
				os.Stdout.Write([]byte("Received: "))
				os.Stdout.Write(buf[:count])
				os.Stdout.Sync() // Гарантируем немедленный вывод
			}
		}
	}
}
