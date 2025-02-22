//go:build linux
// +build linux

package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

// sigaddset устанавливает бит, соответствующий номеру сигнала sig
//func sigaddset(set *unix.Sigset_t, sig int) {
//	index := (sig - 1) / 64
//	offset := uint((sig - 1) % 64)
//	set.Val[index] |= 1 << offset
//}

// createCustomPipe создаёт канал (pipe) для кастомных событий.
func createCustomPipe() (readFD, writeFD int, err error) {
	var FD [2]int
	if err = unix.Pipe(FD[:]); err != nil {
		return -1, -1, err
	}
	readFD, writeFD = FD[0], FD[1]
	if err = unix.SetNonblock(readFD, true); err != nil {
		unix.Close(readFD)
		unix.Close(writeFD)
		return -1, -1, err
	}
	return readFD, writeFD, nil
}

// sendCustomEvent записывает строковое значение в write‑конец кастомного канала.
func sendCustomEvent(customWriteFD int, value string) error {
	_, err := unix.Write(customWriteFD, []byte(value))
	return err
}

func main() {
	epollFD, err := unix.EpollCreate1(0)
	if err != nil {
		panic(err)
	}
	defer unix.Close(epollFD)

	stdinFd := int(os.Stdin.Fd())
	if err := unix.SetNonblock(stdinFd, true); err != nil {
		panic(err)
	}
	stdinEvent := unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(stdinFd)}
	if err := unix.EpollCtl(epollFD, unix.EPOLL_CTL_ADD, stdinFd, &stdinEvent); err != nil {
		panic(err)
	}

	// это не работает, в switch я описал почему в комментариях

	//var signalSet unix.Sigset_t
	//sigaddset(&signalSet, int(unix.SIGINT))
	//
	//// Создаём signalfd для получения SIGINT.
	//signalFD, err := unix.Signalfd(-1, &signalSet, unix.SFD_CLOEXEC)
	//if err != nil {
	//	panic(err)
	//}
	//defer unix.Close(signalFD)
	//
	//sigEvent := unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(signalFD)}
	//if err := unix.EpollCtl(epollFD, unix.EPOLL_CTL_ADD, signalFD, &sigEvent); err != nil {
	//	panic(err)
	//}

	// Создаём pipe для кастомных событий.
	customReadFD, customWriteFD, err := createCustomPipe()
	if err != nil {
		panic(err)
	}
	defer unix.Close(customReadFD)
	defer unix.Close(customWriteFD)

	customEvent := unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(customReadFD)}
	if err := unix.EpollCtl(epollFD, unix.EPOLL_CTL_ADD, customReadFD, &customEvent); err != nil {
		panic(err)
	}

	events := make([]unix.EpollEvent, 1024)
	fmt.Println("Ожидание ввода, custom event или Ctrl+C...")

	for {
		n, err := unix.EpollWait(epollFD, events, -1)
		if err != nil && err != unix.EINTR {
			panic(err)
		}

		for i := 0; i < n; i++ {
			switch events[i].Fd {
			case int32(stdinFd):
				buf := make([]byte, 1024)
				n, err := os.Stdin.Read(buf)
				if err != nil {
					if err == unix.EAGAIN {
						continue
					}
					panic(err)
				}
				if err := sendCustomEvent(customWriteFD, string(buf[:n])); err != nil {
					fmt.Println("Ошибка отправки кастомного события:", err)
				}

				// эта ветка не успеет отработать, если ввести Ctrl+C
				// т.к. сигнал SIGINT будет обработан родительским процессом и
				// программа завершится раньше чем дойдет до этого case
				// т.к. программа привязана к процессу терминала, а это отдельный процесс, который раньше получит sigint

			//case int32(signalFD):
			case int32(customReadFD):
				buf := make([]byte, 1024)
				n, err := unix.Read(customReadFD, buf)
				if err != nil {
					panic(err)
				}
				trimmed := strings.TrimSpace(string(buf[:n]))
				if len(trimmed) > 0 {
					fmt.Printf("Получено кастомное событие: %s\n", trimmed)
				}
				if trimmed == "!quit" {
					// если ввести !quit, то программа завершится корректно
					// и не будет зависать в ожидании ввода
					// и можно тут организовать плавное завершение работы программы
					fmt.Println("Получена команда на выход. Завершение работы.")
					return
				}
			}
		}
	}
}
