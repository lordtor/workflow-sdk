package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	engineURL   string
	httpClient  *http.Client
	serviceName string
	serviceType string
	endpoint    string
	metadata    map[string]string
	heartbeatCh chan struct{}
	stopCh      chan struct{}
	stopOnce    sync.Once
}

type Service struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Endpoint     string            `json:"endpoint"`
	Metadata     map[string]string `json:"metadata"`
	Status       string            `json:"status"`
	HeartbeatAt  time.Time         `json:"heartbeat_at"`
	RegisteredAt time.Time         `json:"registered_at"`
}

// RegistryTokenHeader — заголовок с общим секретом регистрации в реестре.
const RegistryTokenHeader = "X-Registry-Token"

// RegistryRegistrationTokenEnv — переменная окружения с этим секретом. Пока она
// не задана ни на движке, ни у сервиса, регистрация работает как раньше —
// открыто. Заданная переменная включает проверку на обеих сторонах.
const RegistryRegistrationTokenEnv = "REGISTRY_REGISTRATION_TOKEN"

// applyRegistryAuth добавляет секрет регистрации, если он задан окружением.
func applyRegistryAuth(req *http.Request) {
	if token := strings.TrimSpace(os.Getenv(RegistryRegistrationTokenEnv)); token != "" {
		req.Header.Set(RegistryTokenHeader, token)
	}
}

func NewClient(engineURL, serviceName, serviceType, endpoint string, metadata map[string]string) *Client {
	return &Client{
		engineURL: engineURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			// Секрет регистрации едет пользовательским заголовком, а http.Client
			// снимает при редиректе только Authorization/WWW-Authenticate/Cookie.
			// Один 302 на чужой хост — и общий секрет платформы у постороннего,
			// поэтому редиректы не следуем вовсе: у реестра их не бывает.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		serviceName: serviceName,
		serviceType: serviceType,
		endpoint:    endpoint,
		metadata:    metadata,
		heartbeatCh: make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
	}
}

func (c *Client) Register(ctx context.Context) error {
	service := Service{
		ID:           uuid.New().String(),
		Name:         c.serviceName,
		Type:         c.serviceType,
		Endpoint:     c.endpoint,
		Metadata:     c.metadata,
		Status:       "active",
		RegisteredAt: time.Now(),
		HeartbeatAt:  time.Now(),
	}

	url := fmt.Sprintf("%s/api/v1/registry/services", c.engineURL)
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	applyRegistryAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}
	defer resp.Body.Close()

	// 3xx тоже отказ: редиректы мы не следуем (см. CheckRedirect), и молча
	// считать перенаправление успешной регистрацией нельзя.
	if resp.StatusCode >= 300 {
		return fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}

	log.Printf("Service %s registered successfully at %s", c.serviceName, c.endpoint)
	return nil
}

func (c *Client) Heartbeat(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/registry/services/%s/heartbeat", c.engineURL, neturl.PathEscape(c.serviceName))

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}
	applyRegistryAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("heartbeat rejected with status %d (service may need re-registration)", resp.StatusCode)
	}

	return nil
}

func (c *Client) StartHeartbeat(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Heartbeat(ctx); err != nil {
				log.Printf("Heartbeat error: %v", err)
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *Client) Unregister(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/registry/services/%s", c.engineURL, neturl.PathEscape(c.serviceName))

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create unregister request: %w", err)
	}
	applyRegistryAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to unregister service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unregister rejected with status %d", resp.StatusCode)
	}

	return nil
}
