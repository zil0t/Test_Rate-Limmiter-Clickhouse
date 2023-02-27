/*Этот тест проверяет работу метода Allow() структуры IPSubnetRateLimiter.
Он создает новый объект IPSubnetRateLimiter с лимитом 2 запроса в секунду,
выполняет несколько запросов с одним и тем же IP-адресом и проверяет, как они обрабатываются.
Тест ожидает, что первые два запроса будут разрешены, третий - заблокирован из-за превышения лимита,
а четвертый - разрешен после cooldown.*/

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIpLimiter_Allow(t *testing.T) {
	limiter := NewIpLimiter(24, 2, 1*time.Second)

	// создаем тестовый запрос с IP-адресом 192.0.2.1
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "192.0.2.1")

	// первый запрос должен быть разрешен
	if !limiter.Allow("192.0.2.1") {
		t.Error("First request should be allowed")
	}

	// второй запрос должен быть разрешен
	if !limiter.Allow("192.0.2.1") {
		t.Error("Second request should be allowed")
	}

	// третий запрос должен быть заблокирован из-за превышения лимита
	if limiter.Allow("192.0.2.1") {
		t.Error("Third request should be blocked")
	}

	// ждем cooldown
	time.Sleep(2 * time.Second)

	// четвертый запрос должен быть разрешен после cooldown
	if !limiter.Allow("192.0.2.1") {
		t.Error("Fourth request should be allowed after cooldown")
	}
}
