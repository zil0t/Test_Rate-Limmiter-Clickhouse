package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IpLimiter - структура для ограничения количества запросов
// из одной подсети IPv4
type IpLimiter struct {
	prefixLen int                  //prefixLen - длина префикса подсети в битах,
	limit     int                  //limit - максимальное количество запросов в период времени,
	cooldown  time.Duration        //cooldown - временя через которое счетчик запросов сбрасывается до нуля,
	mu        sync.Mutex           //mu - мьютекс для защиты конкурентных доступов,
	counts    map[string]int       //counts - для хранения количества запросов из подсетей
	resets    map[string]time.Time //resets - для хранения времени сброса счетчиков запросов.
}

// NewIpLimiter - функция для создания нового IpLimiter
func NewIpLimiter(prefixLen int, limit int, cooldown time.Duration) *IpLimiter {
	return &IpLimiter{
		prefixLen: prefixLen,
		limit:     limit,
		cooldown:  cooldown,
		counts:    make(map[string]int),
		resets:    make(map[string]time.Time),
	}
}

// Allow для проверки возможности выполнения запроса на основе переданного IP-адреса.
func (limiter *IpLimiter) Allow(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// вычисляем префикс подсети из IP-адреса
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}
	ipNet := &net.IPNet{
		IP:   ipAddr.Mask(net.CIDRMask(limiter.prefixLen, 32)),
		Mask: net.CIDRMask(limiter.prefixLen, 32),
	}
	subnet := ipNet.String()

	// проверяем, не прошло ли время cooldown
	if resetTime, ok := limiter.resets[subnet]; ok && time.Now().Before(resetTime) {
		return false
	}

	// проверяем, не превышен ли лимит запросов
	if count, ok := limiter.counts[subnet]; ok && count >= limiter.limit {
		// устанавливаем время cooldown
		limiter.resets[subnet] = time.Now().Add(limiter.cooldown)
		// сбрасываем счетчик
		limiter.counts[subnet] = 0
		return false
	}

	// увеличиваем счетчик
	limiter.counts[subnet]++
	return true
}

func main() {
	/* Задаём и проверяем переменные окружения
	os.Setenv("PREFIX_LEN", "24")
	os.Setenv("LIMIT", "100")
	os.Setenv("COOLDOWN", "1m")
	env := os.Environ()
	for _, e := range env {
		fmt.Println(e)
	}*/

	// считываем переменные окружения
	prefixLenStr := getEnv("PREFIX_LEN", "24")
	limitStr := getEnv("LIMIT", "100")
	cooldownStr := getEnv("COOLDOWN", "1m")

	// преобразуем переменные в нужный формат
	prefixLen, err := strconv.Atoi(prefixLenStr)
	if err != nil {
		log.Fatalf("Invalid PREFIX_LEN: %s", prefixLenStr)
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		log.Fatalf("Invalid LIMIT: %s", limitStr)
	}
	cooldown, err := time.ParseDuration(cooldownStr)
	if err != nil {
		log.Fatalf("Invalid COOLDOWN_DURATION: %s", cooldownStr)
	}

	// создаем новый IpLimiter
	limiter := NewIpLimiter(prefixLen, limit,
		cooldown)

	// создаем HTTP-сервер
	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := strings.Split(r.Header.Get("X-Forwarded-For"), ", ")[0]

			// проверяем возможность выполнения запроса
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", strconv.Itoa(int(cooldown.Seconds())))
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			// выдаем статический контент
			w.Write([]byte("Hello, World!"))
		}),
	}

	// добавляем handler для сброса лимита по префиксу подсети
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")
		if prefix == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		for subnet := range limiter.counts {
			if strings.HasPrefix(subnet, prefix) {
				delete(limiter.counts, subnet)
				delete(limiter.resets, subnet)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	// запускаем HTTP-сервер
	log.Println("Server started on :8080")
	log.Fatal(server.ListenAndServe())
}

// getEnv - функция для получения значения переменной окружения
// если переменная не задана, используется значение по умолчанию
func getEnv(name string, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		value = defaultValue
	}
	return value
}
