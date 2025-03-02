package main

import (
	"fmt"
	"golang.org/x/net/idna"
	"net/http"
	"strings"
	"sync"
)

func main() {
	var urls = []string{
		"http://ozon.ru",
		"https://ozon.ru",
		"http://google.com",
		"http://somesite.com",
		"http://non-existent.domain.tld",
		"https://ya.ru",
		"http://ya.ru",
		"http://ёёёё",
	}

	rUrls := make([][]rune, 0)
	for _, v := range urls {
		rUrls = append(rUrls, []rune(v))
	}

	//for _, v := range rUrls {
	//	makeAscII(v)
	//}

	first(rUrls)
	second(rUrls)
	//third(urls)
}

func call(url []rune) (*http.Response, error) {
	response, err := http.Get(string(url))
	if err != nil {
		return nil, err
	}
	return response, nil
}

type Result struct {
	response *http.Response
	er       error
}

func callConcurrent(url []rune, result chan *Result) {
	response, err := http.Get(string(url))
	r := &Result{response: response, er: err}
	select {
	case result <- r:
	}
}

func makeAscII(domain []rune) []rune {
	res := string(domain)
	domainAll := strings.Split(res, "://")
	if len(domainAll) > 1 {
		asciiDomain, err := idna.ToASCII(domainAll[1])
		if err != nil {
			fmt.Println("Ошибка преобразования домена:", err)
			return nil
		}
		fmt.Println(domainAll[0] + "://" + asciiDomain)
		return []rune(domainAll[0] + "://" + asciiDomain)
	}
	return nil
}

func first(urls [][]rune) {
	for _, v := range urls {
		//domain := makeAscII(v)
		r, err := call(v)
		if r != nil {
			if r.StatusCode == 200 {
				fmt.Printf("%s - ok \n", string(v))
			} else if r.StatusCode != 200 {
				fmt.Printf("%s - not ok \n", string(v))
			}
		}
		if err != nil {
			fmt.Printf("%s - error \n", err)
		}
	}
}

func second(urls [][]rune) {
	wg := sync.WaitGroup{}
	result := make(chan *Result)
	for _, v := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()

			callConcurrent(v, result)
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

Loop:
	for {
		select {
		case r, ok := <-result:
			//fmt.Println(r)
			if !ok {
				result = nil
				break Loop
			}
			if r.response != nil {
				if r.response.StatusCode == 200 {
					fmt.Printf("%s - ok \n", r.response.Request.URL.Host)
				} else {
					fmt.Printf("%s - not ok \n", r.response.Request.URL.Host)
				}
			} else {
				fmt.Printf("%s - error \n", r.er)
			}
			//default:
			//	time.Sleep(1000 * time.Millisecond)
			//	if result == nil {
			//		break Loop
			//	}
		}

	}
	fmt.Println("done")
}

func third(urls [][]rune) {
	wg := sync.WaitGroup{}
	result := make(chan *Result)
	for _, v := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()

			callConcurrent(v, result)
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	for r := range result {
		if r.response != nil {
			if r.response.StatusCode == 200 {
				fmt.Printf("%s - ok \n", r.response.Request.URL.Host)
			} else {
				fmt.Printf("%s - not ok \n", r.response.Request.URL.Host)
			}
		} else {
			fmt.Printf("%s - error \n", r.er)
		}
	}
}
