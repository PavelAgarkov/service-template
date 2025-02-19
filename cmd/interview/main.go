package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
)

var alphabet = []rune{'a', 'b', 'c', 'd', '1', '2', '3'}

func RecoverPasswordBrutForce(h []byte) string {
	var step int
	genPassword := func(step int) (res string) {
		for {
			key := step % len(alphabet)
			res = string(alphabet[key]) + res
			step = step/len(alphabet) - 1
			if step < 0 {
				break
			}
		}
		return
	}
	for ; ; step++ {
		guess := genPassword(step)
		if bytes.Equal(hashPassword(guess), h) {
			fmt.Printf("step: %d\n", step)
			return guess
		}
	}
}

func hashPassword(in string) []byte {
	h := md5.Sum([]byte(in))
	return h[:]
}

func genPasswords(maxLen int) []string {
	var res []string
	step := 0
	var generate func(prefix string, length int)
	generate = func(prefix string, length int) {
		step++
		if length == 0 {
			res = append(res, prefix)
			return
		}
		for _, r := range alphabet {
			generate(prefix+string(r), length-1)
		}
	}
	for l := 1; l <= maxLen; l++ {
		generate("", l)
	}
	return res
}

// buildRainbowTable создаёт rainbow table (проще – полное сопоставление хэш → пароль)
// для всех паролей длины от 1 до maxLen.
func buildRainbowTable(maxLen int) map[string]string {
	table := make(map[string]string)
	passwords := genPasswords(maxLen)
	for _, pass := range passwords {
		hashBytes := md5.Sum([]byte(pass))
		hashStr := hex.EncodeToString(hashBytes[:])
		table[hashStr] = pass
	}
	return table
}

// RecoverPasswordRainbow ищет пароль по MD5-хэшу в precomputed rainbow table.
func RecoverPasswordRainbow(h []byte, table map[string]string) string {
	hashStr := hex.EncodeToString(h)
	if pass, ok := table[hashStr]; ok {
		return pass
	}
	return ""
}

func main() {
	//rainbowTable := buildRainbowTable(7)
	//fmt.Println(len(rainbowTable), "passwords in the rainbow table")

	for _, exp := range []string{
		"a",
		"12",
		"abc333d",
	} {
		act := RecoverPasswordBrutForce(hashPassword(exp))
		if act != exp {
			fmt.Printf("recovered:", act, "expected:", exp)
		} else {
			fmt.Println(exp, act)
		}
		//h := hashPassword(exp)
		//recovered := RecoverPasswordRainbow(h, rainbowTable)
		//if recovered == exp {
		//	fmt.Printf("Password %q recovered successfully as %q\n", exp, recovered)
		//} else if recovered == "" {
		//	fmt.Printf("Password %q was not found in the rainbow table\n", exp)
		//} else {
		//	fmt.Printf("Recovery mismatch: expected %q, got %q\n", exp, recovered)
		//}

		//Q Как сделать подбор константным по сложности, если мы можем ограничить длину
		//пароля?
		//A Использовать rainbow table.
		//	Q Вычислительная сложность подбора пароля?
		//A O(a^n), а для n-битовой хеш-функции сложность нахождения первого прообраза
		//составляет O(2^n)
		//Q Как атакующий может скомпрометировать криптосистему?
		//A Через timing-attack. Защитой будет сравнение за константное время, кол-во попыток,
		//	ограничение по времени в случае одноразовых паролей из SMS.
		//	Q Как разработчики сервиса могли бы усложнить подбор паролей?
		//A крипто стойкое хеширование, соль, сравнение за константное время
	}
	requests()
}

func requests() {
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

	//first(urls)
	second(urls)
	//third(urls)
}

func call(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return response, nil
}

type Result struct {
	response *http.Response
	er       error
}

func callConcurrent(url string, result chan *Result) {
	response, err := http.Get(url)
	r := &Result{response: response, er: err}
	select {
	case result <- r:
	}
}

func first(urls []string) {
	for _, v := range urls {
		r, err := call(v)
		if err != nil {
			fmt.Printf("%s - not ok \n", v)
		}
		if r != nil {
			if r.StatusCode == 200 {
				fmt.Printf("%s - ok \n", v)
			}
		}
	}
}

func second(urls []string) {
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

func third(urls []string) {
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
