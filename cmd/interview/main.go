package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"golang.org/x/net/idna"
	"hash/crc32"
	"net/http"
	"strings"
	"sync"
	"time"
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

func accum() func(int) int {
	sum := 0
	return func(x int) int {
		sum += x
		return sum
	}
}

type Cache interface {
	Set(k, v string, ttl time.Duration)
	Get(k string) (v string, ok bool)
}

// cacheEntry хранит значение и время истечения срока действия.
// Если expiration.IsZero(), то значение не истекает.
type cacheEntry struct {
	value      string
	expiration time.Time
}

// shardedCache разделяет данные на несколько шардов (частей)
// для повышения параллельности доступа.
type shardedCache struct {
	shards []map[string]*cacheEntry
	locks  []sync.RWMutex
	// Для остановки фоновой уборки можно добавить канал, если потребуется.
}

// NewShardedCache создаёт новый шардированный кеш.
// Рекомендуется выбирать число шардов больше или кратное числу потоков.
func NewShardedCache(ctx context.Context, numShards int, cleanUpInterval time.Duration) Cache {
	if numShards <= 0 {
		numShards = 1
	}
	c := &shardedCache{
		shards: make([]map[string]*cacheEntry, numShards),
		locks:  make([]sync.RWMutex, numShards),
	}
	// Инициализируем каждую map
	for i := 0; i < numShards; i++ {
		c.shards[i] = make(map[string]*cacheEntry)
	}
	// Запускаем фоновую уборку просроченных элементов
	go c.startCleanup(ctx, cleanUpInterval)
	return c
}

// startCleanup запускает периодическую уборку просроченных значений.
func (c *shardedCache) startCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			fmt.Println("cleanup stopped")
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// cleanup проходит по всем шардам и удаляет просроченные элементы.
func (c *shardedCache) cleanup() {
	now := time.Now()
	for i := range c.shards {
		c.locks[i].Lock()
		for key, entry := range c.shards[i] {
			// Если для записи задано время истечения и оно прошло – удаляем
			if !entry.expiration.IsZero() && now.After(entry.expiration) {
				delete(c.shards[i], key)
			}
		}
		c.locks[i].Unlock()
	}
}

// hashKey вычисляет индекс шарда по ключу, используя crc32.
func (c *shardedCache) hashKey(k string) int {
	h := crc32.ChecksumIEEE([]byte(k))
	fmt.Println("hashKey:", int(h)%len(c.shards))
	return int(h) % len(c.shards)
}

// Set сохраняет значение v по ключу k с временем жизни ttl.
// Если ttl == 0, то значение сохраняется без срока истечения.
func (c *shardedCache) Set(k, v string, ttl time.Duration) {
	idx := c.hashKey(k)
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	entry := &cacheEntry{
		value:      v,
		expiration: exp,
	}
	c.locks[idx].Lock()
	c.shards[idx][k] = entry
	c.locks[idx].Unlock()
}

// Get возвращает значение по ключу k.
// Если значение найдено, но уже просрочено, оно удаляется и возвращается false.
func (c *shardedCache) Get(k string) (string, bool) {
	idx := c.hashKey(k)
	c.locks[idx].RLock()
	entry, ok := c.shards[idx][k]
	c.locks[idx].RUnlock()
	if !ok {
		return "", false
	}
	// Проверяем, истёк ли ttl
	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		// Просроченное значение удаляем (с дополнительной блокировкой)
		c.locks[idx].Lock()
		delete(c.shards[idx], k)
		c.locks[idx].Unlock()
		return "", false
	}
	return entry.value, true
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(1 * time.Second)
	}()
	cache := NewShardedCache(ctx, 4, 5*time.Second)
	cache.Set("key1", "value1", 10*time.Second)
	cache.Set("key2", "value2", 10*time.Second)
	fmt.Println(cache.Get("key1")) // "", false
	fmt.Println(cache.Get("key2")) // "", false
	time.Sleep(11 * time.Second)
	fmt.Println(cache.Get("key1")) // "", false
	fmt.Println(cache.Get("key2")) // "", false

	//requests()
	//rainbow()
}

func rainbow() {
	rainbowTable := buildRainbowTable(7)
	fmt.Println(len(rainbowTable), "passwords in the rainbow table")

	for _, exp := range []string{
		"a",
		"12",
		"abc333d",
	} {
		//act := RecoverPasswordBrutForce(hashPassword(exp))
		//if act != exp {
		//	fmt.Printf("recovered:", act, "expected:", exp)
		//} else {
		//	fmt.Println(exp, act)
		//}
		h := hashPassword(exp)
		recovered := RecoverPasswordRainbow(h, rainbowTable)
		if recovered == exp {
			fmt.Printf("Password %q recovered successfully as %q\n", exp, recovered)
		} else if recovered == "" {
			fmt.Printf("Password %q was not found in the rainbow table\n", exp)
		} else {
			fmt.Printf("Recovery mismatch: expected %q, got %q\n", exp, recovered)
		}

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
		//}
	}
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

	rUrls := make([][]rune, 0)
	for _, v := range urls {
		rUrls = append(rUrls, []rune(v))
	}

	//for _, v := range rUrls {
	//	makeAscII(v)
	//}

	first(rUrls)
	//second(rUrls)
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
