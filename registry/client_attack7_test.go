package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Атака 7. Клиент реестра SDK (registry/client.go), коммит 5afee89.
// ─────────────────────────────────────────────────────────────────────────────

// Вектор: общий секрет регистрации утекает на посторонний хост при редиректе.
//
// Предусловия: REGISTRY_REGISTRATION_TOKEN задан (то есть защита «включена»), а
// адрес движка (ENGINE_URL / cfg.Registry.EngineURL) отвечает редиректом — это
// умеет и скомпрометированный/подменённый DNS-ответ, и любой промежуточный
// прокси, и сам движок, если его когда-нибудь переведут на другой хост.
//
// Что доказано: applyRegistryAuth ставит собственный заголовок X-Registry-Token,
// а http.Client по умолчанию следует редиректам и вычищает при смене хоста
// ТОЛЬКО Authorization/WWW-Authenticate/Cookie/Cookie2 (net/http,
// shouldCopyHeaderOnRedirect). Произвольный заголовок переносится как есть,
// поэтому секрет приезжает на чужой хост целиком.
//
// Почему важно: один 302 — и общий секрет всех 11 сервисов у атакующего; после
// этого «закрытая» регистрация открыта для него полностью. Лечится
// CheckRedirect, запрещающим смену хоста (или отключением редиректов вовсе).
func TestAttack7RegistryTokenLeaksAcrossRedirect(t *testing.T) {
	t.Setenv(RegistryRegistrationTokenEnv, "s3cret-token")

	leaked := make(chan string, 4)
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked <- r.Header.Get(RegistryTokenHeader)
		w.WriteHeader(http.StatusCreated)
	}))
	defer attacker.Close()

	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+r.URL.Path, http.StatusFound)
	}))
	defer engine.Close()

	client := NewClient(engine.URL, "vault-service", "external", "http://vault-service:8080", nil)
	// Редирект — не успех: клиент его не следует и обязан сообщить об отказе.
	if err := client.Register(context.Background()); err == nil {
		t.Fatal("редирект принят за успешную регистрацию: клиент обязан вернуть ошибку")
	}

	select {
	case got := <-leaked:
		t.Fatalf("секрет регистрации %q доставлен на посторонний хост %s при редиректе: "+
			"http.Client переносит пользовательские заголовки через смену хоста", got, attacker.URL)
	case <-time.After(time.Second):
		// Ни одного запроса на чужой хост — секрет никуда не уехал.
	}
}

// Вектор: сервис не узнаёт, что его heartbeat отвергнут.
//
// Предусловия: секрет включён на движке, но не доехал до конкретного сервиса
// (устаревший образ, забытая переменная, сервис со своей копией клиента). Движок
// отвечает 401, потому что registryBypassAllowed снял право на bypass.
//
// Что доказано: Heartbeat при статусе >= 400 печатает строчку в log и возвращает
// nil (registry/client.go:122-125). StartHeartbeat ошибку не видит, повторной
// регистрации не делает, метрика не поднимается. Сервис часами считает себя
// живым в реестре, которого он лишился.
//
// Почему важно: это и есть механизм тихой деградации, из-за которого включение
// секрета невозможно откатить по симптомам — никто не сообщает о поломке.
func TestAttack7HeartbeatSwallowsRejection(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer engine.Close()

	client := NewClient(engine.URL, "vault-service", "external", "http://vault-service:8080", nil)

	if err := client.Heartbeat(context.Background()); err == nil {
		t.Fatal("heartbeat отвергнут движком с 401, а клиент вернул nil — " +
			"сервис не узнает, что выпал из реестра")
	}
}

// Вектор: Unregister игнорирует ответ движка целиком.
//
// Предусловия: любые. В боевом роутере движка (internal/api/router.go:215-225)
// маршрута DELETE /api/v1/registry/services/:name вообще нет — то есть ответ на
// снятие с учёта всегда 404.
//
// Что доказано: Unregister не смотрит на StatusCode и всегда возвращает nil.
// Сервис «снялся с учёта», запись в реестре осталась, и потребители продолжают
// брать оттуда endpoint уже мёртвого сервиса — до истечения heartbeat, который
// никто не чистит.
//
// Почему важно: снятие с учёта не работает, а фикс добавил в этот путь ещё и
// отправку секрета — то есть секрет уходит в запрос, который заведомо ничего не
// делает.
func TestAttack7UnregisterIgnoresResponseStatus(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer engine.Close()

	client := NewClient(engine.URL, "vault-service", "external", "http://vault-service:8080", nil)

	if err := client.Unregister(context.Background()); err == nil {
		t.Fatal("движок ответил 404 на снятие с учёта, а клиент отчитался об успехе")
	}
}
